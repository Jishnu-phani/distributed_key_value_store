package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type kvResult struct {
	vv  []VersionedValue
	ok  bool  // the key exists on that replica
	err error // the replica was unreachable or returned an error
}

// fetchFromReplicas asks every replica in parallel and returns as soon as `need`
// replicas have given a REAL answer.
//
// "Key not found" is a real answer (the replica was reachable and says it has
// nothing: err == nil, ok == false). A network or HTTP failure is not: an
// unreachable replica must never count toward quorum, or a read could be
// "confirmed" by a single node, or report "not found" for a key that exists.
//
// If fewer than `need` replicas answer, the returned slice is shorter than
// `need` and the caller MUST treat that as a quorum failure.
func fetchFromReplicas(cluster *Cluster, store *Store, key string, replicas []string, need int) []kvResult {
	results := make(chan kvResult, len(replicas)) // buffered: late goroutines never block

	for _, nodeID := range replicas {
		go func(nodeID string) {
			if nodeID == cluster.selfID {
				vv, ok := store.Get(key)
				results <- kvResult{vv: vv, ok: ok}
				return
			}
			vv, ok, err := cluster.replicateGet(nodeID, key)
			results <- kvResult{vv: vv, ok: ok, err: err}
		}(nodeID)
	}

	var answered []kvResult
	for received := 0; received < len(replicas) && len(answered) < need; received++ {
		if res := <-results; res.err == nil {
			answered = append(answered, res)
		}
	}
	return answered
}

func main() {
	store := NewStore()
	log.Printf("raw NODE_ID from env: %q", os.Getenv("NODE_ID"))
	cluster := NewCluster()

	// Node-to-node calls need an mTLS client, which needs the cert files, so it
	// is built here at startup rather than in a package-level initializer.
	var err error
	httpClient, err = newPeerHTTPClient()
	if err != nil {
		log.Fatalf("failed to build peer HTTP client: %v", err)
	}

	// Internal endpoint: real replicas call THIS directly, no forwarding logic.
	http.HandleFunc("/internal/kv/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/internal/kv/"):]

		switch r.Method {
		case http.MethodGet:
			vv, ok := store.Get(key)
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(vv)

		case http.MethodPut:
			var vv VersionedValue
			if err := json.NewDecoder(r.Body).Decode(&vv); err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			store.Put(key, vv)
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			store.Delete(key)
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Client-facing endpoint: THIS node acts as coordinator for any key.
	http.HandleFunc("/kv/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/kv/"):]
		if key == "" {
			http.Error(w, "key required", http.StatusBadRequest)
			return
		}

		replicas := cluster.ReplicaNodes(key)
		if len(replicas) == 0 {
			http.Error(w, "no replicas available", http.StatusInternalServerError)
			return
		}

		switch r.Method {
		case http.MethodPut:
			results := fetchFromReplicas(cluster, store, key, replicas, cluster.R)
			if len(results) < cluster.R {
				// Without a read quorum we cannot know what this write must
				// causally follow, so refuse before writing anything.
				http.Error(w, "read quorum not reached", http.StatusInternalServerError)
				return
			}
			baseVC := make(VectorClock)
			for _, res := range results {
				if res.ok {
					for _, vv := range res.vv {
						baseVC = baseVC.Merge(vv.Vclock)
					}
				}
			}
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			vc := baseVC.Increment(cluster.selfID)
			vv := VersionedValue{Value: body.Value, Timestamp: time.Now().UnixNano(), Vclock: vc}

			acks := 0
			var mu sync.Mutex
			var wg sync.WaitGroup

			for _, nodeID := range replicas {
				wg.Add(1)
				go func(nodeID string) {
					defer wg.Done()
					var err error
					if nodeID == cluster.selfID {
						store.Put(key, vv) // local write, no HTTP
					} else {
						err = cluster.replicatePut(nodeID, key, vv)
					}
					if err == nil {
						mu.Lock()
						acks++
						mu.Unlock()
					}
				}(nodeID)
			}
			wg.Wait()

			if acks >= cluster.W {
				w.WriteHeader(http.StatusOK)
			} else {
				http.Error(w, "write quorum not reached", http.StatusInternalServerError)
			}

		case http.MethodGet:
			results := fetchFromReplicas(cluster, store, key, replicas, cluster.R)
			if len(results) < cluster.R {
				http.Error(w, "read quorum not reached", http.StatusInternalServerError)
				return
			}

			var best []VersionedValue
			for _, res := range results {
				if res.ok {
					for _, vv := range res.vv {
						best = reconcile(best, vv)
					}
				}
			}

			switch len(best) {
			case 0:
				http.Error(w, "key not found", http.StatusNotFound)
			case 1:
				json.NewEncoder(w).Encode(map[string]string{"key": key, "value": best[0].Value})
			default:
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"key":      key,
					"conflict": true,
					"siblings": best,
				})
			}

		case http.MethodDelete:
			acks := 0
			var mu sync.Mutex
			var wg sync.WaitGroup

			for _, nodeID := range replicas {
				wg.Add(1)
				go func(nodeID string) {
					defer wg.Done()
					if nodeID == cluster.selfID {
						store.Delete(key)
						mu.Lock()
						acks++
						mu.Unlock()
						return
					}
					url := cluster.peers[nodeID] + "/internal/kv/" + key
					req, _ := http.NewRequest(http.MethodDelete, url, nil)
					resp, err := httpClient.Do(req)
					if err == nil && resp.StatusCode == http.StatusOK {
						mu.Lock()
						acks++
						mu.Unlock()
					}
				}(nodeID)
			}
			wg.Wait()

			if acks >= cluster.W {
				w.WriteHeader(http.StatusOK)
			} else {
				http.Error(w, "delete quorum not reached", http.StatusInternalServerError)
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("node %s listening on :8080 (N=%d W=%d R=%d)", cluster.selfID, cluster.N, cluster.W, cluster.R)

	tlsConfig, err := loadServerTLSConfig()
	if err != nil {
		log.Fatalf("failed to load TLS config: %v", err)
	}
	server := &http.Server{Addr: ":8080", TLSConfig: tlsConfig}
	log.Fatal(server.ListenAndServeTLS("", "")) // certs already live inside TLSConfig
}
