# Chapter 5 — The DNS Resolver Pipeline, Line by Line

> Goal: read the heart of the application — the resolution pipeline — end to end,
> and understand the AAAA/NODATA correctness work the refactor introduced.

This is the chapter the whole app exists for. We'll go from a raw network packet
to a DNS reply, through every decision step.

---

## 5.1 A 30-second DNS primer

A DNS **query** asks: "what is the `A` (IPv4) / `AAAA` (IPv6) record for
`example.com`?" A **response** carries answer records and a **response code**:

- `NOERROR` + answers → here's your IP.
- `NOERROR` + **no** answers → **NODATA**: the name exists, but not for this
  record type (e.g. it has an A but no AAAA).
- `NXDOMAIN` → the name does not exist at all.
- `SERVFAIL` → something broke.

The distinction between **NODATA** and **NXDOMAIN** matters and is the crux of
the refactor's correctness fix. Keep it in mind.

---

## 5.2 From packet to message: the handler adapter

Open [`internal/dns/handler.go`](../../internal/dns/handler.go). This thin layer
converts wire-format bytes ↔ a parsed `*dns.Msg` (from the `miekg/dns` library)
and delegates the *thinking* to the resolver.

```go
func (h *Handler) HandleUDP(conn *net.UDPConn, client *net.UDPAddr, data []byte) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {       // bytes -> structured message
		slog.Error("unpack udp query failed", "error", err)
		return
	}
	resp := h.resolver.Resolve(msg, client.IP.String(), "UDP")  // the brain
	if resp == nil {
		return
	}
	out, err := resp.Pack()                          // message -> bytes
	if err != nil {
		slog.Error("pack udp response failed", "error", err)
		return
	}
	if _, err := conn.WriteToUDP(out, client); err != nil {
		slog.Error("write udp response failed", "error", err)
	}
}
```

`HandleTCP` is the same shape with one extra wrinkle: TCP DNS messages are
prefixed with a 2-byte length, so it writes `framed = [hi, lo, ...payload]`.
**Separation of concerns:** the handler knows transport framing; the resolver
knows DNS policy. Neither knows the other's job.

---

## 5.3 The pipeline, top to bottom

Open [`internal/dns/resolver/resolver.go`](../../internal/dns/resolver/resolver.go).
`Resolve` is the entry point and reads like a table of contents:

```go
func (r *Resolver) Resolve(msg *dns.Msg, clientIP, protocol string) *dns.Msg {
	if len(msg.Question) == 0 {
		return nil               // malformed: no question -> drop
	}
	q := msg.Question[0]
	domain := strings.ToLower(strings.TrimSuffix(q.Name, "."))
	qtypeStr := dns.TypeToString[q.Qtype]

	slog.Debug("dns query", "domain", domain, "qtype", qtypeStr, "client", clientIP, "proto", protocol)

	base := models.QueryLog{
		Domain:     domain,
		ClientIP:   clientIP,
		MACAddress: r.mac.Lookup(clientIP),   // cached ARP lookup (Ch 4)
		Protocol:   protocol,
		QueryType:  qtypeStr,
	}

	if resp := r.stepBlocklist(msg, domain, base); resp != nil {
		return resp
	}
	if resp := r.stepSteering(msg, domain, clientIP, qtypeStr, base); resp != nil {
		return resp
	}
	if resp := r.stepCustomRecords(msg, domain, base); resp != nil {
		return resp
	}
	if resp := r.stepCache(msg, domain, base); resp != nil {
		return resp
	}
	return r.stepUpstream(msg, domain, base)
}
```

The design: each step is a method returning `*dns.Msg`. **A non-nil return means
"I handled it, stop here."** A `nil` return means "not my job, try the next
step." This **chain-of-responsibility** structure makes the priority order
explicit and each step independently readable and testable.

Priority order (first match wins):
**blocklist → steering rules → custom records → cache → upstream.**

Notice `domain` is normalized once — lowercased and the trailing `.` stripped —
so every step compares apples to apples. And `base` is the `QueryLog` template
from Chapter 1: each step copies it and fills in the outcome.

---

## 5.4 Step 1 — Blocklist

```go
func (r *Resolver) stepBlocklist(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	if !r.blocklist.IsBlocked(domain) {
		return nil               // not blocked -> next step
	}
	entry := base
	entry.Action = models.ActionBlocked
	entry.ResponseCode = "BLOCKED"
	entry.ResolvedIP = blockedIPv4    // "0.0.0.0"
	entry.AllAnswers = blockedIPv4
	entry.AnswerCount = 1
	r.logger.LogQuery(entry)
	return r.answerBlocked(msg)
}
```

`r.blocklist` is the `Blocklist` *interface* from Chapter 3 — at runtime it's
`*db.DB`, whose SQL handles exact and wildcard matches (Chapter 6). If blocked,
we log and return a blocked answer. We'll see `answerBlocked` in §5.8 — it's
where AAAA-awareness lives.

---

## 5.5 Step 2 — Steering rules

Steering rules let you say "redirect `*.corp.internal` to 10.0.0.5" or "block
queries from 192.168.1.0/24 between 22:00–06:00." From `stepSteering`:

```go
for _, rule := range r.activeRules() {
	if !rule.Enabled || !matchesCondition(rule, domain, clientIP, qtypeStr) {
		continue
	}
	switch rule.ActionType {
	case "Block":
		// ... log + answerBlocked
	case "Redirect":
		return r.steeringRedirect(msg, rule, qtype, base)
	case "Forward":
		if resp := r.steeringForward(msg, rule, base); resp != nil {
			return resp
		}
		// Forward failed: fall through to default resolution.
	}
	break   // first matching rule wins
}
return nil
```

### Caching the rules: avoiding a DB hit per query

`r.activeRules()` doesn't query the database every time. It caches the rule set
for 10 seconds:

```go
func (r *Resolver) activeRules() []models.SteeringRule {
	r.rulesMu.RLock()
	if time.Since(r.rulesLoadedAt) < 10*time.Second {
		rules := r.rulesCache
		r.rulesMu.RUnlock()
		return rules
	}
	r.rulesMu.RUnlock()

	rules := r.steering.Rules()        // refresh from DB
	r.rulesMu.Lock()
	r.rulesCache = rules
	r.rulesLoadedAt = time.Now()
	r.rulesMu.Unlock()
	return rules
}
```

This is the **double-checked, read-mostly cache** pattern: take a cheap `RLock`
to read the cached value; only when stale do we take the exclusive `Lock` to
refresh. Steering rules change rarely but are read on every query, so this trades
10s of staleness for a huge drop in DB load and lock contention.

### Condition matching

[`internal/dns/resolver/matching.go`](../../internal/dns/resolver/matching.go)
holds the pure matching logic — no I/O, trivially testable:

```go
func matchesCondition(rule models.SteeringRule, domain, clientIP, qtypeStr string) bool {
	switch rule.ConditionType {
	case "Domain":     return matchDomain(rule.ConditionValue, domain)
	case "Client IP":  return matchCIDR(rule.ConditionValue, clientIP)
	case "Query Type": /* comma list compare */
	case "Time Range": return matchTimeRange(rule.ConditionValue)
	}
	return false
}
```

`matchDomain` supports `*.suffix` wildcards; `matchCIDR` uses `net.ParseCIDR` to
test subnet membership; `matchTimeRange` parses `"HH:MM-HH:MM"`. Each is a small
total function — the kind of code you want to unit-test exhaustively.

---

## 5.6 The AAAA correctness fix (the important part)

Here's the bug the refactor fixed. Suppose a steering rule redirects
`intranet.local` to the IPv4 address `10.0.0.5`. A client asks for the **AAAA**
(IPv6) record. The *old* code only handled `dns.TypeA`, so an AAAA query fell
through and the client got **no answer at all** — it would hang and retry. The
same bug existed for IPv4-only custom records and for blocked domains.

The fix is `steeringRedirect`:

```go
func (r *Resolver) steeringRedirect(msg *dns.Msg, rule models.SteeringRule, qtype uint16, base models.QueryLog) *dns.Msg {
	if rule.ActionTarget == "" {
		return nil
	}
	target := net.ParseIP(rule.ActionTarget)
	if target == nil {
		return nil
	}
	isV4 := target.To4() != nil

	if (qtype == dns.TypeA && isV4) || (qtype == dns.TypeAAAA && !isV4) {
		// Family matches the question: return the address.
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		entry.ResolvedIP = rule.ActionTarget
		// ... log ...
		return r.answerAddress(msg, rule.ActionTarget, 60)
	}

	// Family MISMATCH (e.g. AAAA asked, only an IPv4 redirect exists): NODATA.
	if qtype == dns.TypeA || qtype == dns.TypeAAAA {
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		r.logger.LogQuery(entry)
		return r.answerNoData(msg)      // empty NOERROR, NOT silence
	}
	return nil
}
```

The key insight: when the question's address family doesn't match what we have,
the correct DNS answer is **NODATA** (an empty `NOERROR`), which tells the client
"this name exists, but it has no IPv6 address — stop asking." That's completely
different from returning nothing (client hangs) or returning the IPv4 address in
an AAAA answer (protocol violation).

The same logic appears in `stepCustomRecords`:

```go
func (r *Resolver) stepCustomRecords(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return nil
	}
	ip, found, existsOtherType := r.records.Lookup(domain, qtype)
	if found {
		// ... return answerAddress
	}
	if existsOtherType {
		// The name has a record of the OTHER family only -> NODATA,
		// so we don't fall through to upstream for a name we own.
		return r.answerNoData(msg)
	}
	return nil
}
```

`Lookup` returns three values: the IP, whether *this* type was found, and
whether the domain has a record of the *other* type. That third return value is
what lets us answer NODATA correctly instead of leaking the query to upstream.
This is why `custom_records` got a `qtype` column in the refactor (Chapter 6).

---

## 5.7 Steps 4 & 5 — Cache and Upstream

### Cache lookup (qtype-aware)

```go
func (r *Resolver) stepCache(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	cached := r.cache.Get(domain, qtype)     // key is (domain, qtype)!
	if cached == nil {
		return nil
	}
	entry := base
	entry.Action = models.ActionCached
	switch {
	case cached.NXDOMAIN:
		entry.ResponseCode = "NXDOMAIN"
		return r.answerNXDOMAIN(msg)
	case cached.NoData:
		entry.ResponseCode = "NOERROR"
		return r.answerNoData(msg)
	default:
		ip := cached.First()
		// ... fill entry, return answerAddress
	}
}
```

The cache key is `(domain, qtype)` (Chapter 4 / 8), so a cached **A** answer for
`example.com` is *not* mistakenly served to an **AAAA** query. The *old* cache
keyed by domain alone and only stored A — another source of the AAAA bug. The
new cache also stores **negative** results (NXDOMAIN and NODATA), so repeated
queries for nonexistent names don't keep hammering upstream.

### Upstream forward + populate cache

When nothing local matches, we forward to the upstream pool and cache the result:

```go
func (r *Resolver) stepUpstream(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	upstream := r.pool.Current()
	start := time.Now()
	resp, err := r.pool.Forward(msg)         // Ch 4: health-checked failover
	latency := millisSince(start)

	if err != nil {
		// ... log ActionError + SERVFAIL
		fail := new(dns.Msg)
		fail.SetRcode(msg, dns.RcodeServerFailure)
		return fail
	}
	r.cacheUpstream(domain, qtype, resp)     // remember the answer
	// ... log ActionForwarded with latency, upstream, answers
	return resp
}
```

`cacheUpstream` decides what kind of entry to store:

```go
func (r *Resolver) cacheUpstream(domain string, qtype uint16, resp *dns.Msg) {
	if resp.Rcode == dns.RcodeNameError {
		r.cache.SetNXDOMAIN(domain, qtype)    // negative cache
		return
	}
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return
	}
	// collect A or AAAA IPs matching the qtype...
	if len(ips) > 0 {
		r.cache.Set(domain, qtype, ips, ttl)
	} else if resp.Rcode == dns.RcodeSuccess {
		r.cache.SetNoData(domain, qtype)      // NODATA cache
	}
}
```

Three outcomes, three cache writes: positive (IPs), NXDOMAIN, or NODATA. Each is
keyed by qtype. This is the cache doing real work to reduce upstream traffic and
latency for *every* answer shape.

---

## 5.8 Building the reply: A vs AAAA vs NODATA vs NXDOMAIN

Open [`internal/dns/resolver/responses.go`](../../internal/dns/resolver/responses.go).
This is where an outcome becomes wire-correct DNS.

```go
func (r *Resolver) answerAddress(req *dns.Msg, ip string, ttl uint32) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return r.answerNoData(req)
	}
	name := req.Question[0].Name
	qtype := req.Question[0].Qtype
	ttlStr := strconv.FormatUint(uint64(ttl), 10)

	var rrText string
	switch {
	case parsed.To4() != nil && qtype == dns.TypeA:
		rrText = name + " " + ttlStr + " IN A " + ip
	case parsed.To4() == nil && qtype == dns.TypeAAAA:
		rrText = name + " " + ttlStr + " IN AAAA " + ip
	default:
		return r.answerNoData(req)      // family mismatch -> NODATA
	}
	if rr, err := dns.NewRR(rrText); err == nil {
		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}
```

`answerAddress` emits an **A** record for IPv4 + A query, an **AAAA** record for
IPv6 + AAAA query, and **NODATA** for any mismatch — the family check lives here
too, as a safety net. The other builders are short and precise:

```go
func (r *Resolver) answerNXDOMAIN(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeNameError)   // NXDOMAIN
	return resp
}

func (r *Resolver) answerNoData(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)                       // NOERROR, empty Answer = NODATA
	return resp
}
```

And `answerBlocked` ties blocking to the same correctness:

```go
func (r *Resolver) answerBlocked(req *dns.Msg) *dns.Msg {
	if r.blockNX {
		return r.answerNXDOMAIN(req)         // configurable: pretend it doesn't exist
	}
	if req.Question[0].Qtype == dns.TypeA {
		return r.answerAddress(req, blockedIPv4, 60)   // A -> 0.0.0.0
	}
	// For AAAA there is no 0.0.0.0 analogue -> NODATA.
	return r.answerNoData(req)
}
```

So a blocked domain returns `0.0.0.0` for A queries, **NODATA** for AAAA, or
**NXDOMAIN** for everything if `block_nxdomain` is on. Every path is now
protocol-correct.

---

## 5.9 The whole journey, one diagram

```
UDP/TCP packet
   │  handler.go: Unpack bytes -> *dns.Msg
   ▼
Resolve(msg, clientIP, proto)         normalize domain, build QueryLog base
   │
   ├─ stepBlocklist     blocked?  → answerBlocked  (0.0.0.0 / NODATA / NXDOMAIN)
   ├─ stepSteering      rule hit? → Block / Redirect(A/AAAA/NODATA) / Forward
   ├─ stepCustomRecords local A/AAAA? → answerAddress, else other-family → NODATA
   ├─ stepCache         (domain,qtype) hit? → positive / NXDOMAIN / NODATA
   └─ stepUpstream      forward via health-checked pool, cache result
   │
   ▼  every step calls r.logger.LogQuery(entry)  (async, Ch 4 + 6)
*dns.Msg response
   │  handler.go: Pack -> bytes, write to client
   ▼
client
```

Every box is a method you can open and a test you can write. That's the payoff of
the pipeline structure.

---

## Exercises

1. **Read a step, predict a reply.** Without running, what does OmniDNS return
   for an **AAAA** query of a domain that has only an IPv4 custom record? (Answer:
   NODATA — `stepCustomRecords`, `existsOtherType` branch.) Now confirm with the
   test `TestCustomRecordAAAAOverA_ReturnsNoData` in
   [`resolver_test.go`](../../internal/dns/resolver/resolver_test.go).
2. **Add a step.** Imagine a "safe search" step that rewrites `google.com` →
   `forcesafesearch.google.com`. Where in `Resolve`'s chain would it go, and what
   would it return? Sketch the method signature.
3. **Trace caching.** Run the server, `dig @127.0.0.1 -p 5354 example.com` twice,
   and watch the query-log `action` go from `forwarded` to `cached`
   (`GET /api/logs`). Then `dig` the same name as `AAAA` — note it's a separate
   cache entry (separate `forwarded` then `cached`).
4. **Negative cache.** `dig` a guaranteed-nonexistent name twice and observe the
   second hit as `cached` with `NXDOMAIN`. Which method stored it? (Answer:
   `cacheUpstream` → `SetNXDOMAIN`.)
5. **Why normalize once?** What would break if `stepBlocklist` compared the raw
   `q.Name` (with trailing dot, mixed case) against lowercased blocklist entries?

[← Chapter 4](04-concurrency.md) · [Chapter 6: Persistence →](06-persistence.md)
