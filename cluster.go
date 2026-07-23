package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
)

type Cluster struct {
	selfID string
	peers  map[string]string
	ring   *HashRing
}

func NewCluster() *Cluster {
	selfID := os.Getenv("NODE-ID")
	if selfID == "" {
		selfID = "node-a"
	}

	ring := NewHashRing(150)
	peers := map[string]string{}

	peerList := os.Getenv("PEERS")
	if peerList != "" {
		for _, peer := range strings.Split(peerList, ",") {
			kv := strings.Split(peer, "=")
			if len(kv) != 2 {
				continue
			}
			peers[kv[0]] = kv[1]
			ring.AddNode(kv[0])
		}
	}
	return &Cluster{
		selfID: selfID,
		peers:  peers,
		ring:   ring,
	}
}

func (c *Cluster) IsOwner(key string) bool {
	owner, ok := c.ring.GetNode(key)
	if !ok {
		return false
	}
	return ok && owner == c.selfID
}

func (c *Cluster) OwnerURL(key string) (string, bool) {
	owner, ok := c.ring.GetNode(key)
	if !ok {
		return "", false
	}
	return c.peers[owner], true
}

func (c *Cluster) Forward(ownerURL, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, ownerURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}
