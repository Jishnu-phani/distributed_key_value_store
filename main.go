package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	store := NewStore()
	log.Printf("raw NODE_ID from env: %q", os.Getenv("NODE_ID"))
	cluster := NewCluster()

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
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			vv := VersionedValue{Value: body.Value, Timestamp: time.Now().UnixNano()}

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
		type result struct {
			vv VersionedValue
			ok bool
		}
		results := make(chan result, len(replicas)) // buffered so late goroutines never block

		for _, nodeID := range replicas {
			go func(nodeID string) {
				if nodeID == cluster.selfID {
					vv, ok := store.Get(key)
					results <- result{vv, ok}
					return
				}
				vv, ok, err := cluster.replicateGet(nodeID, key)
				if err != nil {
					results <- result{VersionedValue{}, false}
					return
				}
				results <- result{vv, ok}
			}(nodeID)
		}

		var best VersionedValue
		found := false
		responded := 0

		// Stop as soon as we hit quorum — don't wait for stragglers.
		for responded < len(replicas) {
			res := <-results
			responded++

			if res.ok && (!found || res.vv.Timestamp > best.Timestamp) {
				best = res.vv
				found = true
			}

			if responded >= cluster.R {
				break // quorum reached — answer now, ignore remaining in-flight replicas
			}
		}

		if responded < cluster.R {
			http.Error(w, "read quorum not reached", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": best.Value})

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
	log.Fatal(http.ListenAndServe(":8080", nil))
}