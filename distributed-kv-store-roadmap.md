# Project Roadmap: Distributed, Security-Hardened Key-Value Store

**A build-to-learn systems project combining distributed systems fundamentals with security engineering**

---

## 1. What This Project Ultimately Does (The Big Picture First)

Before breaking anything down, here's the end state — what you're actually building and why it matters.

You are building a **small database that runs across multiple machines instead of one**, the same fundamental problem that Redis Cluster, DynamoDB, Cassandra, and CockroachDB all solve at massive scale. Your version won't be massive, but it will be *real* — the same mechanisms, just at a scale you can run on a laptop and fully understand.

**The core problem this project solves:** if you store data on one computer and that computer dies, you lose the data (or at minimum, lose availability until it's fixed). The moment you spread data across multiple computers to protect against this, you inherit a cascade of hard problems: *How do you decide which computer holds which piece of data? What happens when two computers disagree about what a value is? How does a computer that was offline for 10 minutes catch back up? How do you stop an attacker from tampering with data on one node and having that corruption silently spread everywhere else?*

Every phase of this project exists to answer one of those questions. By the end, you will have a **multi-node key-value store that**:
- Splits (shards) data intelligently across nodes
- Survives individual node failures without losing data
- Resolves conflicts when multiple nodes write to the same key at nearly the same time
- Detects when nodes silently drift out of sync and repairs them automatically
- Refuses unauthorized access and can prove, after the fact, that its data wasn't tampered with

That last point is what makes *your* version of this project different from the dozen tutorial clones on GitHub — most people build the distributed systems part and stop. You're building the distributed systems part **and** treating it like something an attacker would target, because that's the lens your actual job trains you to use.

---

## 2. Why This Specific Project (Career & Learning Fit)

| Angle | Why it fits |
|---|---|
| **Resume for full-time roles** | Distributed systems + security is a rare, senior-sounding combination for someone still early in their career. Most candidates have one or the other. |
| **Direct continuity with your HPE PSO work** | You already think about vulnerabilities and secure software composition daily. This project asks: "what if the software I'm securing *was* the distributed system?" |
| **Builds on what you already know** | Go, concurrency (goroutines/channels from your earlier learning), containerization internals (MiniDocker), Docker networking (Nextcloud setup) — nothing here starts from zero. |
| **Interview relevance** | "Design a distributed cache," "how would you handle a network partition," "how do you detect tampering in a distributed log" are extremely common system design interview questions. You'll have *built*, not just read about, the answer. |

---

## 3. Feasibility & Cost Analysis — Everything Is Free

This project requires **$0** in mandatory spend. Breakdown:

| Component | Tool | Cost | Notes |
|---|---|---|---|
| Language/runtime | Go | Free | Open source, you already have it set up |
| Local multi-node simulation | Docker / Docker Compose | Free | You already run this for Nextcloud |
| Version control & portfolio hosting | GitHub | Free | Public repo, free tier is enough |
| TLS certificates (security phase) | `mkcert` or OpenSSL self-signed certs | Free | No need for real domain certs for a local project |
| Diagrams for write-up | draw.io / Excalidraw (browser-based) or Mermaid in Markdown | Free | No paid diagramming tool needed |
| Hardware | Your existing laptop (WSL2 Ubuntu) | Free | 4-5 lightweight containers simulating nodes is trivial for any modern laptop — each node is just a small Go binary, not a full OS |
| Blog write-up hosting (optional) | GitHub Pages / dev.to / Hashnode | Free | For the optional accompanying write-up |

**Compute feasibility check:** Each "node" in your cluster is a single Go process (a few MB of RAM, negligible CPU at rest). Running 4-5 of them simultaneously via Docker Compose is comparable to running 4-5 lightweight background apps — well within reach of any laptop from the last several years, including under WSL2's virtualized memory model (which you've already tuned once, during the Nextcloud disk exhaustion troubleshooting).

**Time feasibility check:** ~3 weeks part-time (evenings + weekends around your internship), based on the phase estimates below. No phase requires paid course material — everything referenced is a free tutorial, official documentation, or the original academic papers (also free).

**Conclusion: fully feasible at zero cost, using only tools you already have installed or know how to install.**

---

## 4. Hierarchical Breakdown

```
DISTRIBUTED, SECURITY-HARDENED KEY-VALUE STORE
│
├── PART A — Foundation (Single Node)
│   └── A1. Basic in-memory key-value store
│
├── PART B — Going Distributed (Core, ~1.5–2 weeks)
│   ├── B1. Consistent hashing (sharding data across nodes)
│   ├── B2. Replication (copying data across nodes)
│   ├── B3. Quorum reads/writes (availability during partial failure)
│   └── B4. Vector clocks (resolving conflicting concurrent writes)
│
├── PART C — Self-Healing (Stretch goal, ~1 week)
│   ├── C1. Gossip protocol (nodes discovering each other's state)
│   └── C2. Merkle trees + anti-entropy (detecting & repairing drift)
│
├── PART D — Security Hardening (Your differentiator, ~1 week)
│   ├── D1. TLS between nodes (encrypted, mutually authenticated)
│   ├── D2. Client authentication & authorization (per-key access control)
│   ├── D3. Tamper-evident audit log (hash-chained write history)
│   └── D4. Threat model document
│
└── PART E — Presentation Layer (Ongoing, finalized at the end)
    ├── E1. Architecture diagram
    ├── E2. README with design decisions & trade-offs
    ├── E3. "What broke and how I fixed it" debugging log
    └── E4. Known limitations section
```

Each top-level Part maps to a specific *question* the whole project is answering:

- **Part A** → "How does a key-value store work at all?"
- **Part B** → "How do multiple nodes stay correct together?"
- **Part C** → "How does the system recover on its own, without a human?"
- **Part D** → "How does the system defend itself from being attacked or tampered with?"
- **Part E** → "How do I prove to someone else that I understood all of the above?"

---

## 5. Bite-Sized Progress Steps

Each step below is small enough to finish in a single sitting (1–3 hours). Check them off as you go — this is your literal progress tracker.

### Part A — Foundation
- ✅ A1.1 — Build a basic `GET`/`PUT`/`DELETE` HTTP API backed by an in-memory Go map
- ✅ A1.2 — Add a simple CLI or `curl`-based test script to exercise it
- ✅ A1.3 — Containerize it (one `Dockerfile`, runs as a single node)

*Concept learned: how a minimal key-value store's read/write path works before any distribution is introduced.*

### Part B — Going Distributed
- ✅ B1.1 — Implement consistent hashing to map keys → nodes
- [ ] B1.2 — Set up Docker Compose to run 3–5 node instances simultaneously
- [ ] B1.3 — Route client requests to the correct node based on the hash ring
- [ ] B2.1 — Implement synchronous replication (write to N replicas)
- [ ] B2.2 — Kill a node mid-write manually and observe what happens (this is where the real learning starts)
- [ ] B3.1 — Implement quorum-based reads/writes (e.g. W=2, R=2 out of N=3)
- [ ] B3.2 — Simulate a network partition (e.g. using Docker network rules) and test availability behavior
- [ ] B4.1 — Add vector clocks to each stored value
- [ ] B4.2 — Force a write conflict (two nodes write the same key while partitioned) and resolve it on reconciliation

*Concepts learned: consistent hashing, CAP theorem trade-offs in practice, synchronous replication, quorum principle, vector clocks, conflict resolution.*

### Part C — Self-Healing (Stretch)
- [ ] C1.1 — Implement a basic gossip protocol for node liveness (nodes periodically exchange "I'm alive" + peer state)
- [ ] C1.2 — Detect a failed node automatically via gossip timeout
- [ ] C2.1 — Implement Merkle trees over each node's key range
- [ ] C2.2 — Implement anti-entropy repair (compare Merkle trees between nodes, sync only the differing branches)

*Concepts learned: gossip protocols, failure detection, Merkle trees, anti-entropy/self-healing systems.*

### Part D — Security Hardening (Differentiator)
- [ ] D1.1 — Generate self-signed certs with `mkcert` or OpenSSL for local node-to-node TLS
- [ ] D1.2 — Enforce mutual TLS (mTLS) so nodes only trust each other, not arbitrary connections
- [ ] D2.1 — Add a simple token or API-key based auth layer on the client-facing API
- [ ] D2.2 — Add per-key or per-namespace authorization rules (who can read/write what)
- [ ] D3.1 — Implement a hash-chained audit log (each write's log entry includes the hash of the previous entry)
- [ ] D3.2 — Write a small verifier that walks the chain and detects tampering
- [ ] D4.1 — Write a threat model: what can a malicious/compromised node do, and how does the system detect or limit it?

*Concepts learned: mTLS, authentication vs. authorization, tamper-evident logging (hash chaining — conceptually related to blockchain fundamentals), threat modeling.*

### Part E — Presentation Layer
- [ ] E1.1 — Draw the final architecture (nodes, data flow, replication path, security boundary)
- [ ] E2.1 — Write the README: problem statement, architecture, how to run it locally
- [ ] E2.2 — Write the "design decisions & trade-offs" section (e.g., why quorum over full sync)
- [ ] E3.1 — Write up 2–3 real bugs you hit and how you diagnosed them
- [ ] E4.1 — Write the known limitations section honestly (this signals maturity, not weakness)

---

## 6. How Each Part Fits Into the Bigger Picture

Think of the project as answering one increasingly hard question at each layer:

1. **"Can I store and retrieve a value?"** → Part A gives you this in isolation.
2. **"Can I store it across multiple machines so no single machine is a point of failure?"** → Part B gets you here, and this is the minimum viable version of a genuinely distributed system.
3. **"Can the system notice and fix its own problems without me manually intervening?"** → Part C turns a *distributed* system into a *self-healing* one — this is the difference between a toy and something closer to production-grade.
4. **"Can the system defend itself, and can I prove after the fact that nothing was tampered with?"** → Part D is what turns a distributed-systems project into a distributed-*security*-systems project — this is your specific angle, and it's the part almost no other portfolio project will have.
5. **"Can I explain all of the above clearly to someone who didn't build it?"** → Part E is what actually gets read by a recruiter or hiring manager, and what survives a technical interview follow-up question.

Each part depends on the one before it — you cannot meaningfully do Part D's audit logging without Part B's replication already existing to protect it, and Part C's self-healing has nothing to detect drift *in* without Part B's replicated data existing in the first place.

---

## 7. Suggested Free Learning Resources Per Phase

- **Part A & B (core distributed mechanics):** *"Building a Distributed Key-Value Store in Go: From Single Node to Planet Scale"* — free Substack tutorial covering consistent hashing, quorum, vector clocks, gossip, and Merkle trees in sequence.
- **Deeper consensus (optional, after Part B):** *"Implementing the Raft distributed consensus protocol in Go"* by Phil Eaton — free, builds Raft from scratch, purely educational.
- **Part D (security):** Go's standard `crypto/tls` package documentation (free, official) for mTLS; OWASP's authentication/authorization cheat sheets (free) for the access-control design.
- **CAP theorem & distributed systems theory (background reading):** the original Raft paper *"In Search of an Understandable Consensus Algorithm"* (free PDF) if you want the formal grounding behind what you're implementing.

---

## 8. Realistic Timeline Summary

| Part | Duration | Cumulative |
|---|---|---|
| A — Foundation | 2–3 days | Week 1 |
| B — Core distributed mechanics | 8–10 days | Weeks 1–2.5 |
| D — Security hardening | 5–7 days | Week 3 |
| C — Self-healing (stretch, optional) | +5–7 days | Week 4 (if pursued) |
| E — Write-up | Ongoing, finalized last 2–3 days | Throughout |

**Minimum viable, resume-ready version: Parts A + B + D + E (~3 weeks).**
**Full version with self-healing: add Part C (~4 weeks total).**

---

*This document is meant to be a living checklist — update the checkboxes as you go, and don't hesitate to reorder Part C and D if security ends up interesting you more once you're in it.*
