# Project State: Distributed, Security-Hardened Key-Value Store

*This document is updated after each completed section. It's meant to give a complete picture of the project — what it does, how it's structured, and how each piece works — without needing to read the full build history.*

**Last updated through: Part D, Step D1.1 (in progress — server-side mTLS config)**

---

## 1. What the project can do right now

A client can `PUT`, `GET`, and `DELETE` keys against a **3-node cluster** (`node-a`, `node-b`, `node-c`), where:

- Any node can be contacted for any key — the receiving node acts as **coordinator** and routes internally
- Data is **sharded** across nodes via consistent hashing (no single node holds everything)
- Every key is **replicated** to N=3 nodes, tolerating the loss of any 1 node without losing data or availability
- Reads and writes use **quorum** (W=2, R=2) — fast, available, and mathematically guaranteed to overlap with the latest write (W+R > N)
- Concurrent, conflicting writes (e.g., during a network partition) are **detected honestly via vector clocks**, not silently overwritten via timestamp guessing — the client gets a `409 Conflict` with all conflicting versions visible, rather than silent data loss
- *(In progress)* Node-to-node traffic is being upgraded to require **mutual TLS (mTLS)** — every node will only accept connections from other nodes holding a certificate signed by the cluster's own CA

---

## 2. High-level architecture

```
Client
  │
  ▼
[any node] ── acts as coordinator for the request
  │
  ├── determines the key's N replica nodes (via consistent hash ring)
  ├── fans out the request to all N replicas in parallel
  │     - PUT: writes to each replica's local store; needs W acks
  │     - GET: reads from each replica's local store; needs R responses
  │     - reconciles multiple versions via vector clock comparison
  └── responds to client once quorum is met
```

Every node runs **identical code** — there is no special "leader" node. Which node acts as coordinator for a given request is just whichever one the client happened to contact.

---

## 3. Components and which files own them

| Component | Responsibility | File(s) |
|---|---|---|
| Single-node storage | In-memory key→value(s) store, thread-safe | `store.go` |
| Consistent hashing | Deciding which node(s) own a key | `hashring.go` |
| Cluster/peer config | Node identity, peer list, quorum settings, inter-node HTTP calls | `cluster.go` |
| Causality tracking | Detecting concurrent vs. sequential writes | `vectorclock.go` |
| HTTP API + coordination logic | Client-facing routing, replication fan-out, conflict handling | `main.go` |
| Transport security *(in progress)* | mTLS cert loading and enforcement | `tls.go` |
| Containerization | Building/running each node as a container | `Dockerfile` |
| Cluster orchestration | Running 3 nodes together, networking, env config | `docker-compose.yml` |
| Trust material *(in progress)* | CA + per-node certs/keys | `certs/` (generated via OpenSSL, not code) |

---

## 4. File-by-file breakdown

### `store.go` — the single-node storage engine

**What it does:** holds the actual data for whichever keys this node is a replica for. Thread-safe. Supports storing **multiple concurrent sibling versions** of a key (not just one), and knows how to reconcile a new incoming write against what's already there using vector clock comparison.

**Key types:**
- `VersionedValue{ Value string, Timestamp int64, Vclock VectorClock }` — one version of a key's data
- `Store{ mu sync.RWMutex, data map[string][]VersionedValue }` — a key maps to a **slice** of versions (normally length 1; length 2+ only when there's a genuine unresolved conflict)

**Functions:**

```
NewStore() → *Store
    creates an empty store

Get(key) → ([]VersionedValue, bool)
    read-lock, return whatever slice is stored for key (nil/false if missing)

Put(key, incoming VersionedValue)
    lock (held for the ENTIRE operation — read, decide, write, all atomic)
    existing := data[key]
    data[key] := reconcile(existing, incoming)
    unlock

Delete(key)
    lock, delete(data, key), unlock

reconcile(existingValues []VersionedValue, incoming VersionedValue) → []VersionedValue
    // Compares `incoming` against EVERY existing version using vector clocks.
    survivors := []
    dominated := false
    for each existing in existingValues:
        switch incoming.Vclock.Compare(existing.Vclock):
            case Before:      survivors += existing; dominated = true   // incoming is stale
            case After:       (drop existing — incoming supersedes it)
            case Equal:       (drop existing — duplicate)
            case Concurrent:  survivors += existing                     // real conflict — keep both
    if not dominated:
        survivors += incoming
    return survivors
```

**Why this design:** a single `VersionedValue` field could never represent "two genuinely conflicting values" — the slice type is what makes it *possible* to keep both instead of silently picking one (the core LWW bug this project deliberately avoids).

---

### `hashring.go` — consistent hashing

**What it does:** decides which physical node(s) are responsible for a given key, using consistent hashing with virtual nodes (150 per physical node) so that adding/removing a node only reshuffles a small fraction of keys, not all of them.

**Key type:** `HashRing{ mu, virtualNodes int, ring map[uint32]string, sortedKeys []uint32 }`

**Functions:**

```
NewHashRing(virtualNodes) → *HashRing

hashKey(key) → uint32
    SHA-1 hash, truncated to first 4 bytes as a uint32

AddNode(nodeID)
    for i in 0..virtualNodes:
        place nodeID at hash(nodeID + "#" + i) on the ring
    rebuildSortedKeys()

RemoveNode(nodeID)
    remove all virtual positions for nodeID
    rebuildSortedKeys()

GetNode(key) → (nodeID, found)
    // Single-owner lookup (superseded in practice by GetNodes for replication,
    // but kept for the original consistent-hashing demonstration)
    target := hashKey(key)
    idx := binary search for first ring position >= target (wrap to 0 if past the end)
    return ring[sortedKeys[idx]]

GetNodes(key, n) → []nodeID
    // Real lookup used by the cluster — returns the N distinct physical
    // nodes responsible for a key (walking clockwise, skipping virtual-node
    // duplicates of a node already collected)
    target := hashKey(key)
    startIdx := binary search for first ring position >= target
    walk clockwise from startIdx, collecting DISTINCT physical nodes until n found

rebuildSortedKeys()
    // O(n log n) rebuild of the sorted position list, called on every
    // AddNode/RemoveNode. Writes are rare; reads (GetNode/GetNodes) are
    // the hot path and get O(log n) binary search — deliberate trade-off.
```

---

### `vectorclock.go` — causality tracking

**What it does:** the mechanism that distinguishes "this write is simply newer" from "these two writes genuinely conflicted." A vector clock is a map of `nodeID → counter`.

**Key type:** `VectorClock map[string]int64`

**Functions:**

```
Copy() → VectorClock
    shallow copy into a fresh map (never mutate a clock in place — clocks
    may be referenced by multiple in-flight requests concurrently)

Increment(nodeID) → VectorClock
    newVC := Copy()
    newVC[nodeID] += 1
    return newVC
    // missing nodeID defaults to 0 via Go's map zero-value — no special case needed

Merge(other VectorClock) → VectorClock
    newVC := Copy()
    for each (k, v) in other:
        if v > newVC[k]: newVC[k] = v
    return newVC
    // component-wise max — used to "catch up" a clock with everything
    // a coordinator has seen before incrementing its own counter

Compare(other VectorClock) → Ordering   // Equal | Before | After | Concurrent
    // early-exit implementation: walks the union of both clocks' keys,
    // tracking a single running status. The moment a key's comparison
    // would contradict the status established so far, returns Concurrent
    // immediately (order-independent — correct regardless of Go's
    // randomized map iteration order).
```

**The core rule:** X dominates Y if every counter in X ≥ the corresponding counter in Y, with at least one strictly greater. If neither dominates, they're `Concurrent` — a genuine, undetectable-any-other-way conflict.

---

### `cluster.go` — node identity, peers, and inter-node calls

**What it does:** holds this node's identity and cluster-wide config (read from env vars), computes replica sets via the hash ring, and makes the actual HTTP calls to other nodes.

**Key type:** `Cluster{ selfID string, peers map[string]string, ring *HashRing, N, W, R int }`

**Functions:**

```
NewCluster() → *Cluster
    selfID := env NODE_ID (default "node-a")
    parse env PEERS ("id=url,id=url,...") into peers map, AddNode() each into ring
    N, W, R := env REPLICATION_FACTOR / WRITE_QUORUM / READ_QUORUM (defaults 3/2/2)

envInt(key, default) → int
    small helper: parse an env var as int, or fall back to default

ReplicaNodes(key) → []nodeID
    delegates to ring.GetNodes(key, N)

replicatePut(nodeID, key, value VersionedValue) → error
    if nodeID == selfID: return nil (caller handles locally, no HTTP needed)
    else: HTTP PUT to peers[nodeID] + "/internal/kv/" + key, body = JSON(value)

replicateGet(nodeID, key) → ([]VersionedValue, found, error)
    HTTP GET to peers[nodeID] + "/internal/kv/" + key
    404 → (nil, false, nil)
    other non-200 → (nil, false, error)
    200 → decode JSON array of VersionedValue, return (values, true, nil)
```

**Note:** `replicatePut`/`replicateGet` both send/receive **arrays** of `VersionedValue` now (not single objects) — necessary so that unresolved sibling conflicts survive being replicated between nodes rather than silently collapsing to one during a network hop.

---

### `main.go` — HTTP API and coordination logic

**What it does:** the actual HTTP server. Exposes two endpoint families:
- `/internal/kv/{key}` — raw, no-forwarding access to THIS node's local store (used by peers during replication)
- `/kv/{key}` — the client-facing endpoint; this node acts as coordinator regardless of whether it's actually a replica for the key

**Shared type:**
```
kvResult{ vv []VersionedValue, ok bool }
```

**Shared function:**
```
fetchFromReplicas(cluster, store, key, replicas, need) → []kvResult
    // Fans out to all replicas in parallel via goroutines + a buffered
    // channel (buffer = len(replicas), so late/slow goroutines never
    // block trying to send after we've stopped listening).
    // Returns as soon as `need` results have arrived — does NOT wait
    // for stragglers (important fix made during B3: original version
    // used sync.WaitGroup and blocked until ALL replicas responded).
    launch one goroutine per replica:
        if nodeID == selfID: read from local store directly
        else: call cluster.replicateGet over HTTP
    collect results from the channel until `need` collected (or replicas exhausted)
```

**`/internal/kv/{key}` handler:**
```
GET:    return JSON array of store.Get(key)          (404 if not found)
PUT:    decode JSON array of VersionedValue, store.Put(key, ...) each
DELETE: store.Delete(key)
```

**`/kv/{key}` handler — PUT:**
```
replicas := cluster.ReplicaNodes(key)
existing := fetchFromReplicas(..., need = R)         // read-before-write
baseVC := {}
for each result, each sibling vv in result:
    baseVC = baseVC.Merge(vv.Vclock)                 // account for everything visible
vc := baseVC.Increment(selfID)                       // this write's real causal position
newValue := { Value: body.Value, Timestamp: now, Vclock: vc }

fan out newValue to all N replicas in parallel (local write if self, else replicatePut)
count acks; respond 200 if acks >= W, else 500 "write quorum not reached"
```

**`/kv/{key}` handler — GET:**
```
replicas := cluster.ReplicaNodes(key)
results := fetchFromReplicas(..., need = R)
if len(results) < R: 500 "read quorum not reached"

winners := []
for each result, each sibling vv in result:
    winners = reconcile(winners, vv)                 // same reconcile() logic as Store.Put,
                                                       // folding candidates in one at a time

switch len(winners):
    0: 404 "key not found"
    1: 200, JSON { key, value: winners[0].Value }
    2+: 409 Conflict, JSON { key, conflict: true, siblings: winners }
```

**`/kv/{key}` handler — DELETE:** unchanged from B3 — fans out delete to all replicas, requires W acks. (Not yet updated for multi-sibling semantics — currently just deletes whatever's there; treated as an accepted, documented limitation for now.)

---

### `tls.go` *(in progress — Part D1.1)*

**What it will do:** load this node's certificate/key and the cluster CA, and build a `*tls.Config` requiring mutual TLS authentication for the server; a matching client-side config will be added to `cluster.go`'s `httpClient` next.

**Functions so far:**
```
loadServerTLSConfig() → (*tls.Config, error)
    cert := tls.LoadX509KeyPair(env TLS_CERT_FILE, env TLS_KEY_FILE)
    [NEXT STEP] caPool := x509.CertPool loaded from env TLS_CA_FILE
    [NEXT STEP] return &tls.Config{
                    Certificates: [cert],
                    ClientCAs: caPool,
                    ClientAuth: tls.RequireAndVerifyClientCert,
                }
```

---

## 5. Infrastructure

### `Dockerfile`
Multi-stage build: `golang:1.26-alpine` compiles a static binary; final image is plain `alpine`, copying in only the compiled `kvstore` binary. Minimizes final image size.

### `docker-compose.yml`
Defines `node-a`/`node-b`/`node-c`, each:
- Built from the same `Dockerfile`
- Given a distinct `NODE_ID`, identical `PEERS` list, identical `REPLICATION_FACTOR`/`WRITE_QUORUM`/`READ_QUORUM`
- *(New, Part D)* Mounts `./certs:/certs:ro` and sets `TLS_CERT_FILE`/`TLS_KEY_FILE` (per-node) + `TLS_CA_FILE` (shared)
- Exposed on distinct host ports (8081/8082/8083 → container's 8080)

### `certs/` (generated via OpenSSL, not committed as code)
```
ca-key.pem, ca-cert.pem              — the cluster's private CA
node-{a,b,c}-key.pem                 — each node's private key (stays with that node only)
node-{a,b,c}-cert.pem                — each node's cert, signed by the CA, SAN=DNS:node-{a,b,c}
```

---

## 6. Roadmap status

| Part | Status |
|---|---|
| A — Foundation (single node) | ✅ Done |
| B1 — Consistent hashing + multi-node routing | ✅ Done |
| B2 — Replication | ✅ Done |
| B3 — Quorum reads/writes | ✅ Done |
| B4 — Vector clocks | ✅ Done (unit-tested; real Docker end-to-end conflict test still pending) |
| C — Gossip / anti-entropy (stretch) | Not started |
| D1 — mTLS between nodes | 🔶 In progress (certs generated; Go wiring underway) |
| D2 — Client auth/authorization | Not started |
| D3 — Tamper-evident audit log | Not started |
| D4 — Threat model doc | Not started |
| E — Write-up | Not started |

---

## 7. Known, deliberate limitations (for the eventual write-up)

- **DELETE** doesn't yet participate in the sibling-reconciliation logic the way PUT/GET do.
- **No repair/anti-entropy mechanism** — if a node misses a write (was down) and later comes back, nothing proactively re-syncs it; it'll only catch up the next time that specific key is written or read through it. This is exactly the gap Part C (not yet started) would address.
- **In-memory only** — no disk persistence. A container restart loses all data on that node (protected against by replication across nodes, not by durability on any single node).
- **LWW-by-timestamp is not currently used as a Concurrent-case fallback** — true conflicts are always surfaced to the client via 409, never auto-resolved. (This is a deliberate, stricter choice than the "acceptable fallback" discussed conceptually — worth noting explicitly as a design decision in the write-up.)