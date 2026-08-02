package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Cluster struct {
	selfID string
	peers  map[string]string
	ring   *HashRing
	N, W, R int
}

func NewCluster() *Cluster {
	selfID := os.Getenv("NODE_ID")
	if selfID == "" {
		selfID = "node-a"
	}

	ring := NewHashRing(150)
	peers := map[string]string{}

	peerList := os.Getenv("PEERS")
	if peerList != "" {
		for _, peer := range strings.Split(peerList, ",") {
			kv := strings.SplitN(peer, "=", 2)
			if len(kv) != 2 {
				continue
			}
			peers[kv[0]] = kv[1]
			ring.AddNode(kv[0])
		}
	}

	n := envInt("REPLICATION_FACTOR", 3)
	w := envInt("WRITE_QUORUM", 2)
	r := envInt("READ_QUORUM", 2)

	return &Cluster{selfID: selfID, peers: peers, ring: ring, N: n, W: w, R: r}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

// ReplicaNodes returns the N physical nodes responsible for key.
func (c *Cluster) ReplicaNodes(key string) []string {
	return c.ring.GetNodes(key, c.N)
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

// replicatePut sends a versioned value to one replica's internal endpoint.
func (c *Cluster) replicatePut(nodeID, key string, vv VersionedValue) error {
	if nodeID == c.selfID {
		return nil // handled locally by the caller, not over HTTP
	}
	url := c.peers[nodeID] + "/internal/kv/" + key
	body, _ := json.Marshal(vv)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errStatus(resp.StatusCode)
	}
	return nil
}

// replicateGet fetches a versioned value from one replica's internal endpoint.
func (c *Cluster) replicateGet(nodeID, key string) (VersionedValue, bool, error) {
	url := c.peers[nodeID] + "/internal/kv/" + key
	resp, err := httpClient.Get(url)
	if err != nil {
		return VersionedValue{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return VersionedValue{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return VersionedValue{}, false, errStatus(resp.StatusCode)
	}
	var vv VersionedValue
	if err := json.NewDecoder(resp.Body).Decode(&vv); err != nil {
		return VersionedValue{}, false, err
	}
	return vv, true, nil
}

type errStatus int

func (e errStatus) Error() string { return "unexpected status code" }