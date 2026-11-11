# Testing

`go test -race ./...` is the regression bar. CI runs it on every push and pull request, then builds the binary and runs `plumb version`. `go vet ./...` is part of the release gate below.

## Release gate

Tag a release only when all of these are true:

- `go test -race ./...` and `go vet ./...` pass.
- The release build prints the version passed to `-X github.com/rejisterjack/plumb/internal/cli.version`.
- Journeys 1–4 below pass on that commit. The container runs as uid 65532.
- The README limits still match the code: no disk buffer, no multi-tenant service, replay lost on restart, path TTL 24h, at most 1024 paths.

Journey 5 is manual, once per new relay URL. A green CI run does not prove a project's Stripe signing secret.

## Journeys

1. **First webhook.** Start `plumb relay` with `PLUMB_TOKEN`. Run `plumb connect --port <app>`. POST a Stripe-like body to the printed `public:` URL. Localhost sees the same bytes and `Stripe-Signature`. The provider sees the app's status.
2. **Missed event.** Stop connect. POST again and expect `200` with `X-Plumb-Buffered: 1`. Connect the same path. `plumb replay --last 1` prints `replayed 1` and delivers the body to localhost only.
3. **Second developer.** `alice` and `bob` share one relay. A post to `/h/bob/...` stays off alice's port. A second connect to a live path is rejected.
4. **Container.** Build [Dockerfile](Dockerfile). Run with `PORT` and `PLUMB_TOKEN`. `GET /healthz` is `ok`. Repeat journey 1, then the buffer half of journey 2, through the published port.
5. **Stripe checklist.** The manual list in the README: Dashboard endpoint, `stripe trigger`, signature verification, and replay after the local server was down.

## What the Go tests lock

Relay, in [internal/relay/server_test.go](internal/relay/server_test.go) unless noted:

- Health `GET`, unknown path 404: `TestHealthAndUnknownPath`. Health `HEAD`: `TestHealthHead`.
- Raw body, query string, `Stripe-Signature`, and the app's status and headers: `TestProxyRawBodyAndResponse`.
- Bad token creates no path: `TestTokenMismatch`. `--dev` with an empty token: `TestDevAllowsEmptyToken`. Startup without a token: `TestParseRelayRequiresToken`.
- Body over the cap is 413 and is not stored: `TestBodyTooLarge`.
- Offline tunnel and a drop during forward return `X-Plumb-Buffered: 1`: `TestBufferWhenTunnelDownAndReplay`, `TestTunnelDropDuringForwardIsBuffered`.
- Forward timeout 504, in-flight limit 429: `TestForwardTimeout`, `TestInFlightLimit`.
- One claim per path, isolated bodies: `TestSecondRegisterRejected`, `TestPathsAreIsolated`.
- Idle expiry and the path cap: `TestIdlePathExpires`, `TestPathCapEvictsIdleAndRejectsLive`.
- `..` rejected by the handler: `TestUncleanRequestPath`. The same paths over a real HTTP request, with nothing delivered to localhost: `TestTraversalOverHTTP`. Local path checks: `TestSafePath`.
- Rate limit, ignored `X-Forwarded-For`, visitor cap: `TestRateLimit`, `TestRateLimitIgnoresSpoofedForwardedFor`, `TestLimiterCapsVisitors`.
- Empty upstream 400 keeps its status and headers: `TestLocalErrorStatusIsPreserved`.
- Server deadlines: `TestHTTPServerTimeouts`.

CLI:

- Public URL printed once, socket mode `0600`, silent reconnect, socket replay: `TestProxyRawBodyAndResponse`, `TestClientReconnectsAndReplays`, `TestControlSocketReplay`.
- Replay does not replace a live tunnel: `TestReplayDoesNotStealLiveTunnel`.
- Config file and flag overrides: `TestLoadFileConfig`, `TestConnectFlagsOverrideFile`. Listen address: `TestResolveListenAddr`.
- `plumb version` prints `plumb <version>`: `TestVersionOutput`. Help and unknown command: `TestMainUsage`.
- Replay with the wrong token exits 1, says `unauthorized`, and does not print the token: `TestReplayRejectsBadToken`.

Other:

- Ring keeps the newest entries and copies body and headers: `TestRingOverflowKeepsNewest`, `TestRingCopiesBodyAndHeaders`.
- Terminal log truncates at 64 KiB, marks binary, and strips ASCII control characters. Forwarded bytes are unchanged: `TestLogExchangeTruncatesBody`, `TestLogExchangeReplayAndBinary`, `TestLogExchangeStripsControls`.
- Hop-by-hop headers stripped: `TestCloneForwardStripsHopHeaders`. Token compare: `TestTokenMatch`. Slugs: `TestValidSlug`. Backoff bounds: `TestBackoffStaysInRange`.

These stay as separate tests: raw body round-trip, replay not stealing the tunnel, a single public-URL announcement, the buffer across a drop, path isolation, and socket mode `0600`.
