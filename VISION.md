# Plumb — Webhook Relay for Local Development

> One deploy. Permanent webhook URL. Forward to localhost without reconfiguring Stripe and GitHub on every restart.

**Home:** [plumb.rejisterjack.com](https://plumb.rejisterjack.com)

## One-line idea

A small relay you deploy once plus a local CLI that gives your dev environment a stable public URL for webhooks from Stripe, GitHub, Twilio, and similar services—without ngrok’s changing free-tier URLs.

## The problem

ngrok’s free tier assigns a new URL on every restart. Each restart means updating webhook endpoints in Stripe, GitHub, and every other dashboard—wasted time and broken teammate setups. Paid tunneling works but is overkill for many developers. Plumb is a Go relay on Railway (or your host) with a stable subdomain and a CLI that forwards traffic to any local port.

## What Plumb does

- Relay server with a persistent public subdomain (per deployment)
- Local CLI that opens a tunnel and forwards webhooks to a chosen localhost port
- Multiple developers can use separate paths on the same relay instance
- Replay the last N webhooks on demand after crashes or missed events
- Log each request with headers and body for debugging

## Who it's for

- Developers building Stripe, GitHub App, Twilio, or other webhook-driven integrations locally
- Teams tired of re-sharing ngrok URLs after every restart
- Solo devs who want a set-once webhook URL for local work

## How you use it

1. Deploy the Plumb relay (see README; Railway is the documented path)
2. Install the CLI: `go install github.com/rejisterjack/plumb@latest`
3. Run `plumb connect --relay https://your-relay.example --port 3000`
4. Point third-party webhooks at the relay’s public URL
5. Payloads arrive at your local service in real time

## How it runs

- **Relay:** single Go binary with a public URL (typically Railway)
- **CLI:** Go binary maintaining a WebSocket tunnel to the relay
- Webhooks hit the relay and are forwarded over the tunnel to localhost
- Each team or developer runs their own relay; Plumb is not a shared multi-tenant SaaS

## Tech stack

- Go (relay + CLI)
- WebSocket tunnel (no polling)
- In-memory ring buffer for webhook replay
- No database; relay is stateless aside from the replay buffer

## Success criteria

- Documented one-click or scripted relay deploy
- `go install` for the CLI
- Demo: Stripe (or similar) webhook reaching localhost via stable URL
- `plumb replay --last 5` reproduces recent payloads
- Terminal request log with headers and body
- Open source at [github.com/rejisterjack/plumb](https://github.com/rejisterjack/plumb)

## Limitations & non-goals

- Operators need their own relay instance; document hosting cost (Railway free tier may suffice)
- CLI must reconnect silently when WebSockets drop
- **Local development only**—not for production webhook endpoints
- Competes with ngrok on stability and simplicity, not on every enterprise feature

## Deployment

| Component | Platform |
|-----------|----------|
| Relay server | Railway (or self-host the binary) |
| CLI | GitHub releases, `go install` |
