# Plumb

One deploy. A stable public URL. Webhooks from Stripe, GitHub, and Twilio forwarded to localhost, including after the CLI reconnects.

Home: [plumb.rejisterjack.com](https://plumb.rejisterjack.com)

Plumb is a self-hosted relay for local development. You run your own instance. It is not a shared service, and it is not a production webhook endpoint.

## Quickstart

1. Deploy the relay (Render is the documented path below; Railway works when that account can provision a project) and set `PLUMB_TOKEN`.
2. Install the CLI:

   ```sh
   go install github.com/rejisterjack/plumb@latest
   ```

   Prebuilt binaries for Linux, macOS, and Windows (amd64 and arm64) are on the
   [releases page](https://github.com/rejisterjack/plumb/releases/latest).

3. Open a tunnel to your local server:

   ```sh
   plumb connect --relay https://your-relay.example --port 3000
   ```

4. Copy the printed `public:` URL into the provider dashboard.
5. Payloads arrive at `127.0.0.1:3000` with the original body and signature headers.

`https://your-relay.example/h/<path>/webhooks/stripe` is forwarded to `http://127.0.0.1:3000/webhooks/stripe`. The `/h/<path>` prefix is removed. Query strings are preserved.

## Deploy the relay

The relay is one container. It stores recent webhook bodies in memory and does not use a database. The public URL stays until you delete the service. Restarting the process clears the replay buffer.

Generate a token and set it on the service before the first boot:

```sh
openssl rand -hex 32
```

| Variable | Purpose |
| --- | --- |
| `PLUMB_TOKEN` | Required. Shared secret for `plumb connect`. |
| `PLUMB_BUFFER` | Webhooks kept per path. Default 50. |
| `PLUMB_MAX_BODY` | Maximum body size in bytes. Default 2 MiB. |
| `PLUMB_FORWARD_TIMEOUT` | How long to wait for localhost. Default `15s`. |
| `PORT` | Injected by the host. The process listens on `:$PORT`. |

### Render

Create a free Docker web service from this repo. Point it at the root [`Dockerfile`](Dockerfile), set `PLUMB_TOKEN`, and use health check path `/healthz`. The process listens on `PORT`. Copy the public HTTPS URL into `plumb connect --relay …`.

A free Render service sleeps after about 15 minutes without traffic. The hostname stays; the first request after sleep can take about a minute. Prefer an always-on plan if providers send webhooks while you are idle.

### Railway

The repo includes [`railway.toml`](railway.toml) (`/plumb relay`, health check `GET /healthz`, restart on failure) for accounts that can provision a Railway project:

```sh
# one-time: create/link a project, then set the token before the first boot
railway link
railway variables set PLUMB_TOKEN="$(openssl rand -hex 32)"
railway up
```

Railway injects `PORT`; do not hard-code a listen port.

Useful checks:

```sh
curl -fsS https://your-relay.example/healthz   # expect: ok
```

Restarting or redeploying clears the in-memory replay buffer. That is expected. You can also deploy the Dockerfile on any host that sets `PORT` and `PLUMB_TOKEN`.

Do not set `--dev` on a public host. That flag disables tunnel authentication and is only for localhost.

## Connect

```sh
plumb connect --relay https://your-relay.example --port 3000 --token "$PLUMB_TOKEN"
```

`--token` falls back to `PLUMB_TOKEN`. Omit `--path` and Plumb prints a random unguessable path. The printed public URL does not change when the WebSocket drops; the CLI reconnects silently with the same path and token.

Optional `.plumb.toml` in the working directory. Flags override the file. The token is never read from the file.

```toml
relay = "https://your-relay.example"
port = 3000
path = "alice"
```

Teammates on the same relay use different paths:

```sh
plumb connect --relay https://your-relay.example --port 3000 --path alice
plumb connect --relay https://your-relay.example --port 3000 --path bob
```

A second CLI that tries to claim a path that is already connected is rejected.

Each request is printed in the terminal with its method, path, status, headers, and body. Display of the body stops at 64 KiB. The bytes forwarded to localhost are the original payload, up to the relay's body cap, so signature checks still match.

## Replay

While `plumb connect` is running:

```sh
plumb replay --last 5
```

That re-POSTs the newest buffered webhooks to localhost only. It does not call Stripe or GitHub again.

If the CLI is not running, pull the relay's buffer directly:

```sh
plumb replay --last 5 \
  --relay https://your-relay.example \
  --path alice \
  --port 3000 \
  --token "$PLUMB_TOKEN"
```

A live tunnel keeps its connection. Replay uses a separate short-lived socket. If the tunnel is down when a webhook arrives, the relay still accepts it, returns `200` with `X-Plumb-Buffered: 1`, and holds it for replay. The buffer is in memory and is lost when the relay process exits.

`plumb connect` listens on a per-path socket at `~/.plumb/<path>.sock` (mode `0600`) so `plumb replay` can reach the running process. If that path cannot fit in a Unix socket address, the socket is created under `/tmp/plumb-<uid>/`.

## Security

- The public URL is a secret. Anyone who has it can send webhooks to your machine.
- The tunnel requires `PLUMB_TOKEN`. Inbound provider requests do not, because Stripe and GitHub cannot send that token. The unguessable path is the capability.
- The relay keeps recent raw bodies in memory. Railway logs record method, path, status, size, and duration, not payloads.
- The CLI forwards only to `127.0.0.1` on the port you chose. It refuses relay instructions that are not a local path.
- This is for local development. Do not point production traffic at a Plumb URL.

## Stripe demo checklist

Use this once after a deploy. It is a **manual** checklist, not CI automation. Keep it that way: Stripe Dashboard endpoints, signature verification, and local servers vary per project, so a green pipeline would not prove your webhook path. Run the boxes below when you first deploy or change the relay URL.

- [ ] Relay is up. `curl https://your-relay.example/healthz` returns `ok`.
- [ ] `PLUMB_TOKEN` is set on the relay and in your shell.
- [ ] `plumb connect --relay https://your-relay.example --port 3000` prints a `public:` URL and a `local:` URL.
- [ ] A local server is listening on port 3000 and verifies `Stripe-Signature` over the raw body.
- [ ] In the Stripe Dashboard, add an endpoint at the printed URL plus your route, for example `https://your-relay.example/h/<path>/webhook`.
- [ ] Send a test event from the Dashboard, or run `stripe trigger payment_intent.succeeded`.
- [ ] The terminal shows the event, including `Stripe-Signature`, and Stripe records the status returned by the local server.
- [ ] Stop the local server and send another event. Start the server again and run `plumb replay --last 5`.
- [ ] The missed event is delivered to localhost and is not sent to Stripe a second time.

## Limits

- Each team runs its own relay. Plumb does not host accounts or a multi-tenant control plane.
- Replay history does not survive a relay restart.
- There is no request rewriting, custom domain automation, or production webhook delivery.
- Hosting cost is whatever your relay platform charges for one small always-on service.

**Won't-build (closed):** A durable disk-backed buffer across restarts is intentionally out of scope. The in-memory ring is enough for local webhook recovery; restart clears it by design.

## Development

```sh
go test -race ./...
go build -o plumb .
./plumb relay --dev
```

`--dev` is opt-in. Use it only when the relay is bound to localhost. A public deploy must set `PLUMB_TOKEN` instead.
