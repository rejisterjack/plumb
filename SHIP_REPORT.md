# Ship Readiness Report — Plumb

Plumb is a self-hosted webhook relay for local development. One relay deploy keeps a stable public URL, and the CLI forwards Stripe, GitHub, and Twilio payloads to a localhost port.
A webhook sent to /h/<path>/... is delivered over a WebSocket tunnel with the original body and signature headers. The /h/<path> prefix is removed and the query string is kept.
There is no user account. One PLUMB_TOKEN authenticates the CLI. Provider requests are not authenticated. The unguessable path is the capability.
The relay keeps recent webhook bodies in an in-memory ring. plumb replay re-POSTs them to localhost only. A restart clears the buffer.
The marketing page is the static site in web/, documented at https://plumb.rejisterjack.com. The relay and the CLI are the same Go binary.
Testing finished on 21 Nov 2026 against a local relay, the container image, and the marketing page in Chrome. 59 features passed. The Stripe checklist is blocked because the Stripe CLI is not installed. The decision is GO.

## Feature Table

| ID | Feature | Priority | Status | Bugs Found | Bugs Fixed | UI Issues Fixed | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| RELAY-01 | Health check | P0 | PASSED | 0 | 0 | 0 | GET and HEAD /healthz return ok. Other methods are 405. |
| RELAY-02 | Listen address | P0 | PASSED | 0 | 0 | 0 | Listen address is --addr, then PORT, then :8080. |
| RELAY-03 | Tunnel token required | P0 | PASSED | 0 | 0 | 0 | The relay refuses to start without a token unless --dev is set. |
| RELAY-04 | Dev mode | P1 | PASSED | 0 | 0 | 0 | --dev allows an empty token and prints that authentication is disabled. |
| RELAY-05 | Constant-time token compare | P0 | PASSED | 0 | 0 | 0 | Token compare hashes both sides before a constant-time compare. |
| RELAY-06 | Webhook path and query | P0 | PASSED | 0 | 0 | 0 | The /h/{slug} prefix is removed and the query string is kept. |
| RELAY-07 | Raw body and signature headers | P0 | PASSED | 0 | 0 | 0 | Localhost receives the original body and signature headers. |
| RELAY-08 | Upstream status and headers | P0 | PASSED | 0 | 0 | 0 | The provider receives the local status and the forwardable headers. |
| RELAY-09 | Webhook methods | P1 | PASSED | 0 | 0 | 0 | Seven methods are forwarded. Others are 405 and are not stored. |
| RELAY-10 | Body size cap | P0 | PASSED | 0 | 0 | 0 | An oversized body is 413 and is not stored. |
| RELAY-11 | Buffer when the tunnel is down | P0 | PASSED | 0 | 0 | 0 | A down tunnel returns 200 and X-Plumb-Buffered: 1. |
| RELAY-12 | Buffer when the tunnel drops mid-forward | P0 | PASSED | 0 | 0 | 0 | A drop during forward is buffered instead of failing the provider. |
| RELAY-13 | Forward timeout | P0 | PASSED | 0 | 0 | 0 | A local handler that does not answer in time returns 504. |
| RELAY-14 | In-flight limit | P1 | PASSED | 0 | 0 | 0 | The 33rd overlapping forward on one path is 429. |
| RELAY-15 | Path isolation | P0 | PASSED | 0 | 0 | 0 | Posts to one path never arrive on another path. |
| RELAY-16 | One claim per path | P0 | PASSED | 0 | 0 | 0 | A second connect to a live path is rejected. |
| RELAY-17 | Idle path expiry | P1 | PASSED | 0 | 0 | 0 | An idle disconnected path is dropped after the TTL. |
| RELAY-18 | Path cap | P1 | PASSED | 0 | 0 | 0 | Past the path cap, the oldest idle path is evicted. A full live set is rejected. |
| RELAY-19 | Unsafe request paths | P0 | PASSED | 0 | 0 | 0 | Traversal and non-local paths are rejected and nothing is delivered. |
| RELAY-20 | Rate limit | P0 | PASSED | 0 | 0 | 0 | The limit uses the TCP peer. A spoofed X-Forwarded-For does not reset it. |
| RELAY-21 | Visitor cap | P2 | PASSED | 0 | 0 | 0 | The limiter caps the number of tracked peers. |
| RELAY-22 | HTTP server deadlines | P1 | PASSED | 0 | 0 | 0 | Header, read, write, and idle deadlines are set on the HTTP server. |
| RELAY-23 | Relay log omits the body | P1 | PASSED | 0 | 0 | 0 | Relay logs record method, path, status, size, and duration, not the body. |
| RELAY-24 | Replay over the tunnel | P0 | PASSED | 0 | 0 | 0 | Replay reads the ring on its own socket and does not take the live tunnel. |
| RELAY-25 | Graceful shutdown | P1 | PASSED | 0 | 0 | 0 | A signal closes tunnels and then shuts down the HTTP server. |
| RELAY-26 | Register timeout | P1 | PASSED | 0 | 0 | 0 | The first tunnel message must be a register, and it must arrive within 10 seconds. |
| RELAY-27 | Tunnel ping | P2 | PASSED | 0 | 0 | 0 | Ping and pong keep an idle tunnel from looking dead. |
| RELAY-28 | Hop-by-hop headers | P1 | PASSED | 0 | 0 | 0 | Hop-by-hop headers are stripped. Signature headers are kept. |
| RELAY-29 | Empty upstream response | P1 | PASSED | 0 | 0 | 0 | An empty local 400 is preserved. A generated error uses the status text. |
| RELAY-30 | Unknown routes | P1 | PASSED | 0 | 0 | 0 | Unknown URLs and unknown slugs are 404. |
| CLI-01 | Connect flags and config file | P0 | PASSED | 0 | 0 | 0 | Flags override .plumb.toml. The file is optional. |
| CLI-02 | Token is not read from the file | P0 | PASSED | 0 | 0 | 0 | The token is never read from .plumb.toml. |
| CLI-03 | Random path | P0 | PASSED | 0 | 0 | 0 | An omitted path becomes a random unguessable slug. |
| CLI-04 | Public URL printed once | P0 | PASSED | 0 | 0 | 0 | The public and local URLs are printed once. |
| CLI-05 | Localhost-only forward | P0 | PASSED | 0 | 0 | 0 | The CLI forwards only to 127.0.0.1 and does not follow redirects. |
| CLI-06 | Silent reconnect | P0 | PASSED | 0 | 0 | 0 | A dropped tunnel reconnects with the same path and does not print a new URL. |
| CLI-07 | Terminal log | P1 | PASSED | 0 | 0 | 0 | The terminal log truncates at 64 KiB and strips controls. Forwarded bytes stay raw. |
| CLI-08 | Control socket | P0 | PASSED | 0 | 0 | 0 | The per-path control socket is mode 0600. |
| CLI-09 | Replay while connected | P0 | PASSED | 0 | 0 | 0 | Replay through the running socket re-POSTs to localhost and prints replayed N. |
| CLI-10 | Replay while disconnected | P0 | PASSED | 0 | 0 | 0 | A disconnected replay pulls the buffer and delivers it only to localhost. |
| CLI-11 | Replay does not steal the tunnel | P0 | PASSED | 0 | 0 | 0 | Replay leaves the live tunnel in place. |
| CLI-12 | Replay rejects a bad token | P0 | PASSED | 0 | 0 | 0 | A bad replay token exits 1, says unauthorized, and does not print the token. |
| CLI-13 | Version | P1 | PASSED | 0 | 0 | 0 | plumb version prints plumb and the build version. |
| CLI-14 | Help and unknown command | P2 | PASSED | 0 | 0 | 0 | help exits 0. An unknown command exits 2. |
| CLI-15 | CLI body cap | P1 | PASSED | 0 | 0 | 0 | The CLI body cap comes from --max-body or PLUMB_MAX_BODY. |
| CLI-16 | Reconnect backoff | P2 | PASSED | 0 | 0 | 0 | Reconnect backoff stays between the minimum and the maximum. |
| BUF-01 | In-memory ring | P0 | PASSED | 0 | 0 | 0 | The ring keeps the newest entries and copies the body and headers. |
| WEB-01 | Marketing home | P1 | PASSED | 0 | 0 | 1 | Faint text was 3.93:1 and is now 6.59:1. No overflow at 360, 768, 1024, or 1440. |
| WEB-02 | Page navigation | P1 | PASSED | 0 | 0 | 1 | Keyboard focus is a 2px accent outline. How it works lands on #how. The console had no errors. |
| WEB-03 | Site security headers | P1 | PASSED | 0 | 0 | 0 | The static site sends nosniff, DENY, a strict referrer policy, and a locked-down CSP. |
| WEB-04 | Title, description, and icon | P1 | PASSED | 0 | 0 | 0 | The document has a title, description, Open Graph URL, and an SVG icon. |
| WEB-05 | Responsive layout | P1 | PASSED | 0 | 0 | 0 | The layout stacks on a narrow screen and does not overflow. |
| WEB-06 | Reduced motion | P2 | PASSED | 0 | 0 | 0 | The blink animation is skipped when reduced motion is requested. |
| DEVOPS-01 | GitHub Actions CI | P0 | PASSED | 0 | 0 | 0 | CI runs the race tests, builds a binary, and prints the version. |
| DEVOPS-02 | Container image | P0 | PASSED | 1 | 1 | 0 | Go 1.27.3 was affected by GO-2026-6859. The image now builds with 1.27.4, runs as uid 65532, and govulncheck is clean. |
| DEVOPS-03 | Railway config | P1 | PASSED | 0 | 0 | 0 | Railway starts /plumb relay and checks /healthz. |
| DEVOPS-04 | Release binaries | P1 | PASSED | 0 | 0 | 0 | GoReleaser builds six platform archives and stamps the tag as the version. |
| TEST-01 | Go race suite | P0 | PASSED | 0 | 0 | 0 | go test -race ./... passed on Go 1.27.4. govulncheck reported no vulnerabilities. |
| TEST-02 | Local journeys | P0 | PASSED | 0 | 0 | 0 | A live webhook, a missed event, two paths, and the same checks through the container all passed. |
| TEST-03 | Stripe checklist | P1 | BLOCKED | 0 | 0 | 0 | BLOCKED. The Stripe CLI is not installed, so the Dashboard checklist was not run. |

## Gaps

| ID | Gap | Decision | Reason |
| --- | --- | --- | --- |
| GAP-01 | Durable replay across a restart | DEFER | The README lists a disk-backed buffer as won't-build. The in-memory ring is the product. |
| GAP-02 | Accounts and a multi-tenant control plane | DEFER | Each team runs its own relay. There is no signup. |
| GAP-03 | Request rewriting | DEFER | The README lists request rewriting as absent. |
| GAP-04 | Custom domain automation | DEFER | The public URL is the host the operator deploys. The repo does not provision domains. |
| GAP-05 | Production webhook delivery | DEFER | The README says Plumb is for local development and is not a production endpoint. |
| GAP-06 | A relay dashboard | DEFER | The operator surface is the CLI and the static page. There is no admin UI in the source. |
| GAP-07 | A light theme | DEFER | web/styles.css is one dark theme. There is no theme toggle. |
| GAP-08 | Billing | DEFER | Hosting cost is the operator's platform bill. Plumb does not charge. |

Deferred gaps are not feature rows. There is no BUILD gap.

## Per-Feature Sections

### RELAY-01 Health check

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go
- **Expected behavior:** GET and HEAD /healthz return 200 and the text ok. Any other method is 405. The body of a HEAD response is empty.
- **Acceptance:**
- [x] GET /healthz is 200 and the body is ok.
- [x] HEAD /healthz is 200 and the body is empty.
- [x] POST /healthz is 405.
- **Test cases:**
- Normal GET and HEAD.
- A blank path and a long path are not this route.
- A non-GET method is 405.
- No session exists.
- Two parallel GETs both return ok.
- No external service.
- Empty versus a busy relay.
- Refresh the request.
- **UI checks:** No screen. Check the response status, content type, and body.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-02 Listen address

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/parse.go, Dockerfile, railway.toml
- **Expected behavior:** The relay listens on --addr, otherwise :$PORT, otherwise :8080. The Docker entrypoint is /plumb relay.
- **Acceptance:**
- [x] An empty --addr with PORT set listens on that port.
- [x] With neither, the default is :8080.
- [x] A public host is not started in this step.
- **Test cases:**
- PORT and --addr.
- An empty PORT.
- A non-numeric PORT.
- No auth on the listen flag itself.
- Start once.
- No external service.
- **UI checks:** No screen.
- **Dependencies:** RELAY-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-03 Tunnel token required

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/parse.go, internal/relay/server.go
- **Expected behavior:** plumb relay refuses to start when PLUMB_TOKEN and --token are empty, unless --dev is set.
- **Acceptance:**
- [x] A missing token exits 2 and names PLUMB_TOKEN.
- [x] A token of spaces is treated as set and is not trimmed into empty by this check.
- [x] --dev with an empty token is allowed.
- **Test cases:**
- A 32-byte hex token.
- Empty, whitespace, and a 10_000-character token.
- The token is a string flag.
- No login.
- Start twice.
- No external service.
- **UI checks:** No screen. Confirm the error text does not echo a supplied token.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-04 Dev mode

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/cli/cli.go, internal/relay/auth.go
- **Expected behavior:** --dev with an empty token prints that tunnel authentication is disabled. An empty token is accepted only when Dev is set. The README says not to use --dev on a public host.
- **Acceptance:**
- [x] The warning is printed.
- [x] A register with an empty token succeeds only in dev.
- [x] A non-empty wrong token still fails when a token is configured.
- **Test cases:**
- Dev on localhost.
- --dev together with a token.
- No role system.
- One process.
- **UI checks:** No screen.
- **Dependencies:** RELAY-03
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-05 Constant-time token compare

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/auth.go
- **Expected behavior:** tokenMatch hashes both sides with SHA-256 and compares the digests in constant time. A wrong token does not create a path.
- **Acceptance:**
- [x] Equal tokens match.
- [x] Different lengths do not match.
- [x] A bad token leaves no path.
- **Test cases:**
- The configured token.
- Empty, one extra character, and a unicode token.
- No session.
- **UI checks:** No screen.
- **Dependencies:** RELAY-03
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-06 Webhook path and query

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/path.go
- **Expected behavior:** /h/{slug}/webhooks/stripe?x=1 forwards as /webhooks/stripe?x=1. The slug is 1 to 64 letters, digits, hyphen, or underscore. A missing slug is 404.
- **Acceptance:**
- [x] The prefix is removed.
- [x] The query is preserved.
- [x] An invalid slug is 404.
- **Test cases:**
- A normal Stripe path.
- A trailing slash, an empty rest path, and a long query.
- A slug with a slash, a space, or a character outside the allowed set.
- No login.
- Two paths at once.
- **UI checks:** No screen.
- **Dependencies:** RELAY-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-07 Raw body and signature headers

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/tunnel/forward.go
- **Expected behavior:** The bytes and headers the provider sent, including Stripe-Signature, are what localhost receives. The CLI does not re-sign the body.
- **Acceptance:**
- [x] The local body equals the posted body.
- [x] Stripe-Signature is present.
- [x] The provider sees the local status.
- **Test cases:**
- A JSON body.
- An empty body, a body of spaces, and a body of non-English text.
- A binary body and a body that contains a script tag.
- No login.
- Two posts in parallel.
- A closed local port.
- **UI checks:** No screen. Compare bytes.
- **Dependencies:** CLI-05
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-08 Upstream status and headers

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/httpheader/headers.go
- **Expected behavior:** The status and forwardable headers from localhost are returned to the provider. Hop-by-hop headers are not copied back.
- **Acceptance:**
- [x] A 201 and a custom header come back.
- [x] Connection and Transfer-Encoding are not copied.
- **Test cases:**
- 200, 204, and 400 from localhost.
- An empty header value.
- A header with a newline.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-09 Webhook methods

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/server.go
- **Expected behavior:** GET, HEAD, POST, PUT, PATCH, DELETE, and OPTIONS are accepted. Other methods are 405 and are not stored.
- **Acceptance:**
- [x] POST is accepted.
- [x] TRACE is 405 and is not in the buffer.
- **Test cases:**
- Each allowed method.
- An empty method is not sent by the client.
- TRACE and a custom verb.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-10 Body size cap

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/cli/parse.go
- **Expected behavior:** A body over PLUMB_MAX_BODY, default 2 MiB, returns 413 and is not stored. Zero or negative sizes are rejected at startup.
- **Acceptance:**
- [x] One byte over the cap is 413.
- [x] The ring length does not grow.
- [x] A body at the cap is stored.
- **Test cases:**
- A body at the cap.
- An empty body and a body one byte over.
- A non-numeric PLUMB_MAX_BODY.
- No login.
- Two oversized posts.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-11 Buffer when the tunnel is down

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/buffer/ring.go
- **Expected behavior:** A webhook for a known path with no connected CLI returns 200 and X-Plumb-Buffered: 1. The entry stays in the ring.
- **Acceptance:**
- [x] The header is 1.
- [x] The body is recoverable with replay.
- [x] An unknown path is 404 and is not stored.
- **Test cases:**
- A normal POST while disconnected.
- An empty body.
- A path that was never claimed.
- No login.
- Several buffered posts.
- Restart is out of this item.
- **UI checks:** No screen.
- **Dependencies:** BUF-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-12 Buffer when the tunnel drops mid-forward

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/relay/ws.go
- **Expected behavior:** If the WebSocket closes while a forward is in flight, the provider still gets 200 and X-Plumb-Buffered: 1.
- **Acceptance:**
- [x] The provider status is 200.
- [x] The entry remains in the ring.
- **Test cases:**
- Close the socket during the local handler.
- A slow local handler.
- No login.
- One in-flight request.
- **UI checks:** No screen.
- **Dependencies:** RELAY-11
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-13 Forward timeout

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/cli/parse.go
- **Expected behavior:** When localhost does not answer before PLUMB_FORWARD_TIMEOUT, the provider gets 504. The default is 15s.
- **Acceptance:**
- [x] A hung local handler returns 504.
- [x] A fast handler still returns its own status.
- **Test cases:**
- A handler that sleeps past the timeout.
- A timeout of a few milliseconds in a unit test.
- A non-duration env value is a startup error.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-14 In-flight limit

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/ws.go
- **Expected behavior:** Each path allows 32 forwards at once. One more returns 429 while the others are still running.
- **Acceptance:**
- [x] The extra request is 429.
- [x] The in-flight ones still complete.
- **Test cases:**
- 33 overlapping slow handlers.
- A burst after they finish.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-15 Path isolation

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/ws.go, internal/relay/server_test.go
- **Expected behavior:** A post to /h/bob is not delivered to the CLI registered as alice, and bob's body is not in alice's buffer.
- **Acceptance:**
- [x] Alice's local server sees only alice.
- [x] Bob's buffer does not contain alice's body.
- **Test cases:**
- Two slugs.
- A slug that differs by one character.
- No login.
- Parallel posts.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-16 One claim per path

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/ws.go
- **Expected behavior:** A second tunnel register for a path that already has a connection receives the error path in use. The first connection stays up.
- **Acceptance:**
- [x] The second register is rejected.
- [x] The first CLI still receives the next webhook.
- **Test cases:**
- Two connects with the same path.
- A reconnect after the first exits.
- A wrong token does not displace the owner.
- Parallel registers.
- **UI checks:** No screen.
- **Dependencies:** RELAY-05
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-17 Idle path expiry

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/ws.go, internal/cli/parse.go
- **Expected behavior:** A path with no tunnel and no webhook for PLUMB_PATH_TTL, default 24h, is dropped. A connected path, or one still receiving webhooks, is kept.
- **Acceptance:**
- [x] An idle disconnected path is removed after the TTL.
- [x] A connected path is kept.
- [x] A path that receives a webhook has its timer refreshed.
- **Test cases:**
- A short TTL in a unit test.
- A path at the boundary.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-11
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-18 Path cap

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/ws.go
- **Expected behavior:** The relay holds at most PLUMB_MAX_PATHS, default 1024. A new connect past that evicts the oldest idle path. If every path is live, the new connect is rejected.
- **Acceptance:**
- [x] An idle path is evicted.
- [x] A full set of live paths rejects the new one with too many paths.
- **Test cases:**
- A cap of 1 or 2 in a unit test.
- The default is not started as 1024 live sockets in this step.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-17
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-19 Unsafe request paths

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server.go, internal/tunnel/forward.go
- **Expected behavior:** A request path that is not clean, including .. and backslashes, is 400 and is not forwarded. The CLI also refuses a tunnel instruction that is not a local path.
- **Acceptance:**
- [x] /h/slug/../secret is 400.
- [x] Localhost receives nothing.
- [x] A URL with :// in the forwarded path is refused by the CLI.
- **Test cases:**
- A normal path.
- .., an encoded dot-dot, a backslash, and a scheme.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-20 Rate limit

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/ratelimit.go
- **Expected behavior:** Inbound webhook requests are limited to 100 per second per TCP peer, with a burst of 200. X-Forwarded-For does not create a new bucket.
- **Acceptance:**
- [x] Traffic past the burst is 429.
- [x] A spoofed X-Forwarded-For stays in the same bucket.
- **Test cases:**
- A burst under 200.
- A burst over 200.
- Different forwarded-for values from one peer.
- No login.
- Parallel requests.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-21 Visitor cap

- **Status:** PASSED
- **Priority:** P2
- **Files:** internal/relay/ratelimit.go
- **Expected behavior:** The limiter caps how many distinct peer keys it tracks so a flood of source addresses cannot grow the map without bound.
- **Acceptance:**
- [x] The test TestLimiterCapsVisitors passes when the suite is run.
- **Test cases:**
- Many distinct peer keys.
- The same peer repeated.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-20
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-22 HTTP server deadlines

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/server.go
- **Expected behavior:** ReadHeaderTimeout is 10s, ReadTimeout is 60s, WriteTimeout covers that plus one forward plus 15s, IdleTimeout is 120s, and MaxHeaderBytes is 1 MiB.
- **Acceptance:**
- [x] TestHTTPServerTimeouts matches those values.
- [x] A header block over 1 MiB is rejected.
- **Test cases:**
- The configured durations.
- A slow header.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-02
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-23 Relay log omits the body

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/server.go
- **Expected behavior:** The relay log line records path, method, status, byte count, and duration. It does not record the webhook body.
- **Acceptance:**
- [x] A distinctive body string is absent from the log.
- [x] The status and byte count are present.
- **Test cases:**
- A body containing a fake secret.
- An empty body.
- No login.
- **UI checks:** No screen. Read the log text.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-24 Replay over the tunnel

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/ws.go, internal/tunnel/replay.go
- **Expected behavior:** A WebSocket register with mode replay and a valid token returns the last N buffered items and does not claim the path. A live tunnel stays connected.
- **Acceptance:**
- [x] The batch contains the stored body.
- [x] The live tunnel still receives the next webhook.
- **Test cases:**
- last 1 and last 5.
- last 0 is rejected by the CLI before the socket.
- A wrong token.
- A second replay while a tunnel is up.
- **UI checks:** No screen.
- **Dependencies:** RELAY-11
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-25 Graceful shutdown

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/cli/cli.go
- **Expected behavior:** SIGINT or SIGTERM closes tunnels and shuts the HTTP server within the forward timeout. ListenAndServe returning ErrServerClosed exits 0.
- **Acceptance:**
- [x] A signal stops the process.
- [x] In-flight forwards are canceled by CloseTunnels.
- **Test cases:**
- One signal.
- A second signal is not required for this item.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-02
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-26 Register timeout

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/ws.go
- **Expected behavior:** A tunnel socket that does not send a register message within 10 seconds is closed. The first message must be a register.
- **Acceptance:**
- [x] A late register does not claim a path.
- [x] A non-register first message gets expected register.
- **Test cases:**
- An immediate register.
- Silence.
- A request message first.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-05
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-27 Tunnel ping

- **Status:** PASSED
- **Priority:** P2
- **Files:** internal/relay/ws.go, internal/tunnel/client.go
- **Expected behavior:** The tunnel uses ping and pong so an idle socket is not mistaken for a dead one before the idle timeout. The CLI default idle timeout is 60 seconds.
- **Acceptance:**
- [x] An idle connected tunnel is still registered inside the idle window.
- [x] A socket that stops answering is dropped.
- **Test cases:**
- An idle minute.
- A half-open socket.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-28 Hop-by-hop headers

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/httpheader/headers.go
- **Expected behavior:** CloneForward drops hop-by-hop headers such as Connection, Keep-Alive, and Transfer-Encoding on the way to localhost and on the way back.
- **Acceptance:**
- [x] Those headers are absent on the local request.
- [x] Stripe-Signature is kept.
- **Test cases:**
- A normal signature header.
- A Connection header that names another header.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-29 Empty upstream response

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/server.go
- **Expected behavior:** A real localhost response with status 400 and an empty body keeps 400 and its headers. A nil body is reserved for an error this process generated.
- **Acceptance:**
- [x] 400 with an empty body stays 400.
- [x] A generated 413 still uses the plain error text.
- **Test cases:**
- Empty 400.
- Empty 200.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-08
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### RELAY-30 Unknown routes

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/relay/server.go
- **Expected behavior:** A URL that is not /healthz, /tunnel, or /h/{slug}/... is 404. A slug that was never claimed is also 404.
- **Acceptance:**
- [x] / is 404.
- [x] /h/never-connected/hook is 404.
- **Test cases:**
- A normal missing URL.
- A very long URL.
- A URL with a script tag in the path.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-01 Connect flags and config file

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/parse.go, internal/cli/config.go
- **Expected behavior:** plumb connect reads --relay, --port, --path, and --token. .plumb.toml may set relay, path, and port. A flag overrides the file. A missing file is fine.
- **Acceptance:**
- [x] Flags win over the file.
- [x] A missing relay or port exits 2.
- [x] A port outside 1 to 65535 is rejected.
- [x] A bad line in the file names the line number.
- **Test cases:**
- A complete file.
- Empty values, a quoted value, and a comment.
- A port of 0, 65536, and a word.
- No login.
- **UI checks:** No screen.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-02 Token is not read from the file

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/config.go, internal/cli/parse.go
- **Expected behavior:** The token comes from --token or PLUMB_TOKEN. A token key in .plumb.toml is ignored.
- **Acceptance:**
- [x] A token only in the file does not authenticate.
- [x] PLUMB_TOKEN is used when the flag is omitted.
- **Test cases:**
- The env token.
- A file that contains token = secret.
- No login.
- **UI checks:** No screen. Confirm the process does not print the token.
- **Dependencies:** CLI-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-03 Random path

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/tunnel/client.go, internal/protocol/protocol.go
- **Expected behavior:** Omitting --path assigns a random slug of letters and digits that ValidSlug accepts. The same process keeps that slug across reconnects.
- **Acceptance:**
- [x] Two connects without --path get different slugs.
- [x] The slug matches the public URL.
- **Test cases:**
- Omitted path.
- An explicit path.
- A 65-character path and a path with a slash are rejected.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-04 Public URL printed once

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/tunnel/client.go, internal/tunnel/url.go
- **Expected behavior:** After register, the CLI prints public: and local: once. A reconnect does not print them again. The public URL ends with /h/{path}/.
- **Acceptance:**
- [x] The first register prints both lines.
- [x] A second register in the same process does not.
- [x] local: uses 127.0.0.1 and the chosen port.
- **Test cases:**
- A normal connect.
- A reconnect.
- No login.
- **UI checks:** No screen. Read stdout.
- **Dependencies:** CLI-03
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-05 Localhost-only forward

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/tunnel/forward.go
- **Expected behavior:** Every webhook is sent to http://127.0.0.1:{port}. Redirects are not followed. The Host header is the local address. X-Forwarded-Host and X-Plumb-Original-Url are added.
- **Acceptance:**
- [x] The dial target is 127.0.0.1.
- [x] A 302 from localhost is returned as 302, not followed.
- [x] A path with .. is refused.
- **Test cases:**
- A 200 from a local test server.
- An empty body.
- A redirect.
- A path of //evil and a path containing ://.
- No login.
- A closed port.
- **UI checks:** No screen.
- **Dependencies:** RELAY-07
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-06 Silent reconnect

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/tunnel/client.go
- **Expected behavior:** When the WebSocket drops, the CLI connects again with the same path and token. It does not print a new public URL.
- **Acceptance:**
- [x] The path is unchanged after a drop.
- [x] The public line appears once.
- **Test cases:**
- Drop the socket once.
- Several drops.
- A relay that refuses the token stops with an error rather than spinning forever.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-04
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-07 Terminal log

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/tunnel/log.go
- **Expected behavior:** Each exchange prints method, path, status, duration, headers, and body. The display stops at 64 KiB. ASCII control characters except newline and tab are replaced. Invalid UTF-8 is marked binary. The bytes sent to localhost are not sanitized.
- **Acceptance:**
- [x] A 64 KiB plus one body shows the truncation mark.
- [x] A bell character is not written raw.
- [x] The local server still receives the bell byte.
- **Test cases:**
- A short JSON body.
- An empty body, a binary body, and a body with ESC.
- A header value with a control character.
- No login.
- **UI checks:** The terminal text is the UI. Check alignment of the summary line and that the body cannot move the cursor.
- **Dependencies:** CLI-05
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-08 Control socket

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/tunnel/control.go
- **Expected behavior:** plumb connect listens on ~/.plumb/{path}.sock with mode 0600. If the path cannot fit in a socket address, the socket is under /tmp/plumb-{uid}/.
- **Acceptance:**
- [x] The mode is 0600.
- [x] The socket is removed when connect exits.
- **Test cases:**
- A short path.
- A path long enough to miss the sun_path limit.
- No login.
- Two connects on different paths.
- **UI checks:** No screen. Check the file mode.
- **Dependencies:** CLI-03
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-09 Replay while connected

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/cli.go, internal/tunnel/control.go
- **Expected behavior:** plumb replay --last 5 talks to the running connect socket and re-POSTs that many buffered webhooks to localhost. It prints replayed N.
- **Acceptance:**
- [x] A buffered event is delivered once.
- [x] replayed matches the count.
- [x] --last 0 exits 2.
- **Test cases:**
- last 1 and last 5.
- last 0 and a negative last.
- A missing socket with no relay flags exits 2.
- No login.
- Two replays in a row.
- **UI checks:** No screen.
- **Dependencies:** CLI-08
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-10 Replay while disconnected

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/cli.go, internal/tunnel/replay.go
- **Expected behavior:** When connect is not running, replay needs --relay, --path, --port, and the token. It pulls the buffer and delivers it locally. It does not call the original provider.
- **Acceptance:**
- [x] The local server receives the stored body.
- [x] No request is made to the provider host.
- [x] A missing flag exits 2.
- **Test cases:**
- A full flag set.
- A missing --port.
- A wrong token.
- No login.
- A relay that is down.
- **UI checks:** No screen.
- **Dependencies:** RELAY-24
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-11 Replay does not steal the tunnel

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/ws.go, internal/tunnel/client.go
- **Expected behavior:** A replay socket closes after the batch. The existing connect remains the path owner.
- **Acceptance:**
- [x] The next live webhook still reaches the original connect.
- [x] Replay does not print a new public URL.
- **Test cases:**
- Replay during an active connect.
- A second replay.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-24
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-12 Replay rejects a bad token

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/cli/cli.go, internal/relay/ws.go
- **Expected behavior:** A replay with the wrong token exits 1, says unauthorized, and does not print the token.
- **Acceptance:**
- [x] The exit code is 1.
- [x] stderr contains unauthorized.
- [x] The supplied token is absent from the output.
- **Test cases:**
- The right token.
- A wrong token, an empty token, and a token that contains the word unauthorized.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-10
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-13 Version

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/cli/cli.go, .goreleaser.yaml
- **Expected behavior:** plumb version prints plumb and the version string. A local go build prints dev. A release build injects the tag.
- **Acceptance:**
- [x] plumb version prints plumb dev for a local build.
- [x] -v and --version do the same.
- [x] The usage text is not mixed into the version line.
- **Test cases:**
- No arguments.
- An extra argument is ignored by this command path or rejected, and the test records which.
- No login.
- **UI checks:** No screen.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-14 Help and unknown command

- **Status:** PASSED
- **Priority:** P2
- **Files:** internal/cli/cli.go
- **Expected behavior:** No arguments, an unknown command, and a bad flag exit 2 and print usage. help, -h, and --help exit 0.
- **Acceptance:**
- [x] An unknown command names the command.
- [x] help exits 0.
- **Test cases:**
- help.
- A blank command and a command of emoji.
- No login.
- **UI checks:** No screen.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-15 CLI body cap

- **Status:** PASSED
- **Priority:** P1
- **Files:** internal/cli/parse.go, internal/tunnel/forward.go
- **Expected behavior:** plumb connect and plumb replay read PLUMB_MAX_BODY, and --max-body overrides it. A non-positive value is rejected. A local response over the cap is an error.
- **Acceptance:**
- [x] --max-body 10 overrides the env.
- [x] --max-body 0 exits 2.
- [x] A local response over the cap is not treated as a successful delivery.
- **Test cases:**
- The default.
- A small cap.
- A non-numeric env value.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### CLI-16 Reconnect backoff

- **Status:** PASSED
- **Priority:** P2
- **Files:** internal/tunnel/client.go
- **Expected behavior:** Reconnect waits grow from a minimum to a maximum and stay inside that range.
- **Acceptance:**
- [x] TestBackoffStaysInRange passes when the suite is run.
- [x] The wait never exceeds the max.
- **Test cases:**
- Several failures.
- A canceled context stops the wait.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-06
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### BUF-01 In-memory ring

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/buffer/ring.go
- **Expected behavior:** Each path keeps the newest PLUMB_BUFFER entries, default 50. Add copies the body and headers. Last returns the newest n, oldest first. The ring is gone when the process exits.
- **Acceptance:**
- [x] The 51st entry drops the oldest.
- [x] Mutating the caller's slice does not change the stored body.
- [x] A new process has an empty ring.
- **Test cases:**
- One entry.
- Fifty-one entries.
- A body that is later overwritten by the caller.
- No login.
- **UI checks:** No screen.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### WEB-01 Marketing home

- **Status:** PASSED
- **Priority:** P1
- **Files:** web/index.html, web/styles.css
- **Expected behavior:** The page explains the relay, shows the four steps, the install command, and the deploy notes. The hero heading is the stable-URL sentence. Download and GitHub links point at the repository.
- **Acceptance:**
- [x] The heading is present.
- [x] The go install command is present.
- [x] The GitHub links use https://github.com/rejisterjack/plumb.
- **Test cases:**
- Load /.
- A missing asset.
- No form, so there is no submit.
- Logged-out is the only state.
- **UI checks:** Check alignment, spacing, and overflow. The page is one dark theme. Check 360, 768, 1024, and 1440. Hover and focus the links. The console should be quiet.
- **Dependencies:** None.
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 1
- **Files Changed:** web/styles.css
- **Notes:** Faint text was 3.93:1 and is now 6.59:1. No overflow at 360, 768, 1024, or 1440.

### WEB-02 Page navigation

- **Status:** PASSED
- **Priority:** P1
- **Files:** web/index.html
- **Expected behavior:** The header links to #how, #install, #deploy, and the GitHub repository. The brand link returns to the top.
- **Acceptance:**
- [x] Each anchor lands on its section.
- [x] The brand link goes to /.
- **Test cases:**
- Click each link.
- Keyboard focus through the header.
- No login.
- Two clicks.
- **UI checks:** The header stays visible. On a narrow width the in-page links hide and the GitHub link remains. Focus is visible.
- **Dependencies:** WEB-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 1
- **Files Changed:** web/styles.css
- **Notes:** Keyboard focus is a 2px accent outline. How it works lands on #how. The console had no errors.

### WEB-03 Site security headers

- **Status:** PASSED
- **Priority:** P1
- **Files:** web/vercel.json
- **Expected behavior:** Every response sends nosniff, a strict referrer policy, X-Frame-Options DENY, and a CSP that allows only same-origin style and images. Favicon and CSS are cached for one hour.
- **Acceptance:**
- [x] The four security headers are present.
- [x] CSP default-src is none.
- [x] The cache header is on the CSS and the icon.
- **Test cases:**
- GET /.
- GET /styles.css.
- A missing path.
- No login.
- **UI checks:** No interactive control. Confirm the page still renders under that CSP.
- **Dependencies:** WEB-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### WEB-04 Title, description, and icon

- **Status:** PASSED
- **Priority:** P1
- **Files:** web/index.html, web/favicon.svg
- **Expected behavior:** The title and description describe a stable webhook URL. og:url is https://plumb.rejisterjack.com. The icon is /favicon.svg.
- **Acceptance:**
- [x] The title element matches the heading's product name.
- [x] og:url is the documented home.
- [x] /favicon.svg returns 200 and an SVG.
- **Test cases:**
- The home document.
- A missing icon path.
- No login.
- **UI checks:** The icon is visible in the tab. Text does not overflow the title.
- **Dependencies:** WEB-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### WEB-05 Responsive layout

- **Status:** PASSED
- **Priority:** P1
- **Files:** web/styles.css
- **Expected behavior:** Below 760px the header keeps the brand and the GitHub link, and the split sections stack. The terminal sample does not force a page-wide horizontal scroll.
- **Acceptance:**
- [x] At 360 the page has no horizontal overflow.
- [x] At 768, 1024, and 1440 the sections stay inside the viewport.
- **Test cases:**
- The four widths.
- Zoomed text.
- No login.
- Refresh at each width.
- **UI checks:** Check alignment, spacing, the sticky header, the terminal block, and focus. There is one dark theme.
- **Dependencies:** WEB-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### WEB-06 Reduced motion

- **Status:** PASSED
- **Priority:** P2
- **Files:** web/styles.css
- **Expected behavior:** The blink animation runs only when the user has not asked for reduced motion.
- **Acceptance:**
- [x] With prefers-reduced-motion: reduce, the blink animation is not applied.
- **Test cases:**
- The default motion setting.
- The reduced setting.
- No login.
- **UI checks:** The terminal chrome stays readable in both settings.
- **Dependencies:** WEB-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### DEVOPS-01 GitHub Actions CI

- **Status:** PASSED
- **Priority:** P0
- **Files:** .github/workflows/ci.yml
- **Expected behavior:** On every push and pull request, CI runs go test -race ./..., builds the binary with version ci, and runs plumb version.
- **Acceptance:**
- [x] The workflow file contains those three steps.
- [x] The Go version comes from go.mod.
- [x] This step does not push.
- **Test cases:**
- Read the workflow.
- Run the same commands locally only in step 2.
- No login.
- **UI checks:** No screen.
- **Dependencies:** TEST-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### DEVOPS-02 Container image

- **Status:** PASSED
- **Priority:** P0
- **Files:** Dockerfile, .dockerignore
- **Expected behavior:** The image builds with CGO disabled from the Go version in the Dockerfile and runs as the distroless nonroot user. The entrypoint is /plumb and the command is relay.
- **Acceptance:**
- [x] The Dockerfile uses the nonroot base.
- [x] The process is not root.
- [x] GET /healthz inside the container is ok when step 2 runs it.
- **Test cases:**
- A local docker build in step 2.
- PORT and PLUMB_TOKEN set.
- A missing token.
- No login.
- **UI checks:** No screen.
- **Dependencies:** RELAY-02
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 1
- **Bugs Fixed:** 1
- **UI Issues Fixed:** 0
- **Files Changed:** go.mod, Dockerfile
- **Notes:** Go 1.27.3 was affected by GO-2026-6859. The image now builds with 1.27.4, runs as uid 65532, and govulncheck is clean.

### DEVOPS-03 Railway config

- **Status:** PASSED
- **Priority:** P1
- **Files:** railway.toml
- **Expected behavior:** railway.toml builds the Dockerfile, starts /plumb relay, checks GET /healthz, and restarts on failure up to 5 times.
- **Acceptance:**
- [x] The start command and health path match the README.
- [x] This step does not deploy.
- **Test cases:**
- Read the file.
- No login.
- **UI checks:** No screen.
- **Dependencies:** DEVOPS-02
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### DEVOPS-04 Release binaries

- **Status:** PASSED
- **Priority:** P1
- **Files:** .goreleaser.yaml, .github/workflows/release.yml
- **Expected behavior:** A tag v* builds linux, darwin, and windows for amd64 and arm64, injects the tag as the version, and writes checksums. The workflow does not run on a branch push.
- **Acceptance:**
- [x] Six targets are listed.
- [x] The ldflags set internal/cli.version.
- [x] No tag is pushed in this step.
- **Test cases:**
- Read the config.
- A tag that does not match v* is not a trigger.
- No login.
- **UI checks:** No screen.
- **Dependencies:** CLI-13
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** Passed when exercised on 21 Nov 2026.

### TEST-01 Go race suite

- **Status:** PASSED
- **Priority:** P0
- **Files:** internal/relay/server_test.go, internal/cli/cli_test.go, internal/buffer/ring_test.go
- **Expected behavior:** go test -race ./... is the regression bar named in TESTING.md. It covers the relay, the CLI, the ring, headers, and the protocol.
- **Acceptance:**
- [x] The command exits 0.
- [x] A failure is not deleted to make it pass.
- **Test cases:**
- The full suite.
- The race detector.
- No login.
- Run once, not in parallel with a live relay that shares nothing.
- **UI checks:** No screen.
- **Dependencies:** DEVOPS-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** go test -race ./... passed on Go 1.27.4. govulncheck reported no vulnerabilities.

### TEST-02 Local journeys

- **Status:** PASSED
- **Priority:** P0
- **Files:** TESTING.md, README.md
- **Expected behavior:** Journeys 1 through 4 are a first webhook, a missed event plus replay, two developers, and the same checks through the container. The container user is uid 65532.
- **Acceptance:**
- [x] Journey 1 delivers the same bytes and Stripe-Signature.
- [x] Journey 2 returns X-Plumb-Buffered: 1 and replayed 1.
- [x] Journey 3 keeps bob off alice's port and rejects a second connect.
- [x] Journey 4 repeats the live and buffered checks in the container.
- **Test cases:**
- A Stripe-like body.
- An empty body and a body with non-English text.
- A second connect.
- A stopped CLI.
- No login.
- Refresh is a second POST.
- **UI checks:** The CLI transcript is the UI for these journeys. Check that the public URL, status, and body are readable.
- **Dependencies:** TEST-01
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** A live webhook, a missed event, two paths, and the same checks through the container all passed.

### TEST-03 Stripe checklist

- **Status:** BLOCKED
- **Priority:** P1
- **Files:** README.md
- **Expected behavior:** Journey 5 is the manual Stripe checklist: health, token, connect, a local verifier, a Dashboard endpoint, a test event, and replay after the local server was down. CI does not run it.
- **Acceptance:**
- [x] The checklist stays manual.
- [ ] Step 2 runs it only if a Stripe test key and CLI are already available. Otherwise it is blocked with that reason.
- **Test cases:**
- The README boxes.
- A missing Stripe CLI.
- No login.
- Do not send a live secret to the report.
- **UI checks:** The provider dashboard is outside this repo. Record only the local transcript.
- **Dependencies:** TEST-02
- **Suspected bugs:** None recorded from the source read.
- **Bugs Found:** 0
- **Bugs Fixed:** 0
- **UI Issues Fixed:** 0
- **Files Changed:** none
- **Notes:** BLOCKED. `stripe` is not on PATH, so the Dashboard event was not sent.

## Assumptions

- The product is the Go module at the repo root plus the static site in web/. The relay is not a database app.
- https://plumb.rejisterjack.com is the documented marketing home. This step did not open it and did not read a live token.
- Step 2 will run go test -race ./... and the local journeys. The Stripe checklist runs only when a Stripe test CLI is already available.
- Browser checks for the static page use Chrome DevTools. No new browser harness is added.
- --dev is a localhost switch. A public relay in step 2 uses a generated PLUMB_TOKEN and is not left running afterward.

## Risks

- The public /h/<path> URL is a secret. Anyone who has it can POST to the connected machine.
- Behind Render or Railway the rate-limit peer is the platform proxy, so the limit covers the whole relay.
- The replay ring is in memory. A restart or a redeploy drops it.
- A free Render service sleeps after about 15 minutes. The first webhook after sleep can be slow.
- Anyone who has the relay token can open a tunnel. There is no second role.
- A reconnect keeps the same path. It does not ask the provider to change the webhook URL.

## Whole-Product Check

- `go test -race ./...` passed on Go 1.27.4. `govulncheck ./...` reported no vulnerabilities. Go 1.27.3 had GO-2026-6859 in `net/http`, which this relay calls through `MaxBytesReader`.
- The container built from this Dockerfile runs `/plumb relay` as uid 65532. `GET /healthz` returned `ok`. A webhook through the container kept the body and `Stripe-Signature`. A missed event returned `X-Plumb-Buffered: 1`, and `plumb replay --last 1` printed `replayed 1`. A restart dropped the path.
- Rate limit: 220 posts from one client returned 210 responses of 404 and 10 of 429. A different `X-Forwarded-For` stayed on that limit.
- A second connect while the path was full printed `too many paths`. A reconnect printed the public URL once. The control socket was mode `0600`. A bad replay token exited 1 and did not print the token.
- Chrome showed the marketing page with no horizontal overflow at 360, 768, 1024, and 1440. Faint text is 6.59:1. Keyboard focus is a 2px accent outline. The console had no errors. Reduced motion removes the blink animation.
- https://plumb.rejisterjack.com returned the security headers from `web/vercel.json`. This pass did not deploy the contrast change.

## Totals

- Features: 60
- Passed: 59
- Blocked: 1
- P0: 32
- P1: 23
- P2: 5
- Bugs found: 1
- Bugs fixed: 1
- UI issues fixed: 2

## Still Blocked

TEST-03 Stripe checklist. `stripe` is not installed, and this pass did not send a Dashboard event. Unblock by installing the Stripe CLI and running the README checklist against a relay you control.

## Known Limitations

- Deferred, and not feature rows: a disk-backed buffer, accounts, request rewriting, custom domain automation, production delivery, a relay dashboard, a light theme, and billing.
- The replay buffer is in memory and clears on restart.
- Behind Render or Railway the rate-limit peer is the platform proxy.
- The public `/h/<path>` URL is a secret.
- The marketing site currently deployed does not yet include the contrast and focus changes from this pass.

## Launch Checklist

- Set `PLUMB_TOKEN` before the first boot. Do not set `--dev` on a public host.
- Deploy the Dockerfile and point the platform health check at `GET /healthz`.
- Build release binaries from a `v*` tag. The Go version for that build is 1.27.4.
- Copy the printed `public:` URL into the provider once. Replay stays on localhost.
- Keep the relay off production traffic.

## Decision

GO. 59 features passed. TEST-03 stays blocked until someone runs the manual Stripe checklist. The relay, the CLI, the container, and the marketing page passed the checks in this report. Go 1.27.4 clears GO-2026-6859.
