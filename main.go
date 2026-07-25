package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	store := NewStore()
	log.Printf("raw NODE_ID from env: %q", os.Getenv("NODE_ID"))
	cluster := NewCluster()

	http.HandleFunc("/debug/owner/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/debug/owner/"):]
		owner, _ := cluster.ring.GetNode(key)
		fmt.Fprintf(w, "self=%s owner=%s isOwner=%v\n", cluster.selfID, owner, cluster.IsOwner(key))
	})

	http.HandleFunc("/kv/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/kv/"):]
		if key == "" {
			http.Error(w, "Key is required", http.StatusBadRequest)
			return
		}

		if !cluster.IsOwner(key) {
			ownerURL, ok := cluster.OwnerURL(key)
			if !ok {
				http.Error(w, "Owner not found", http.StatusInternalServerError)
				return
			}

			body, _ := io.ReadAll(r.Body)
			resp, err := cluster.Forward(ownerURL, r.Method, r.URL.Path, body)
			if err != nil {
				http.Error(w, "Failed to forward request: "+err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		switch r.Method {
		case http.MethodGet:
			value, ok := store.Get(key)
			if !ok {
				http.Error(w, "Key not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"key": key, "value": value})

		case http.MethodPut:
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			store.Put(key, body.Value)
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			store.Delete(key)
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("kv store listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}


// Why cant we take random nodes for quorum, why only go circularly in the ring?
// What a read does, and the LWW caveat worth flagging honestly

// The coordinator queries R replicas in parallel and needs to pick one value to return if they disagree (e.g., one replica hasn't caught up yet). For now we'll use last-write-wins by timestamp — return whichever replica reported the newest write time. This is a real simplification: timestamps can be wrong or clocks can drift across machines, and LWW can silently drop a legitimately concurrent write. This is exactly the gap B4 (vector clocks) exists to fix properly — so we're building something honest-but-imperfect now, on purpose, so the vector clock upgrade later actually means something instead of being abstract.
// shouldnt we check for minimum 2 reads regardless, why would we have to come to a conclusion? is this for recovery of the cluster?