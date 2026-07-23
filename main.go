package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func main() {
	store := NewStore()
	cluster := NewCluster()

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
