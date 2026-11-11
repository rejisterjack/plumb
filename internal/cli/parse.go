package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func parseConnect(args []string, file fileConfig) (connectOpts, error) {
	maxBody, err := envInt("PLUMB_MAX_BODY", 2<<20)
	if err != nil {
		return connectOpts{}, err
	}
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	relay := fs.String("relay", "", "relay base URL")
	path := fs.String("path", "", "tunnel path; a random slug is used when omitted")
	port := fs.Int("port", 0, "local port to forward to")
	token := fs.String("token", os.Getenv("PLUMB_TOKEN"), "tunnel token (or PLUMB_TOKEN)")
	body := fs.Int("max-body", maxBody, "maximum body bytes (or PLUMB_MAX_BODY)")
	if err := fs.Parse(args); err != nil {
		return connectOpts{}, err
	}
	if *body < 1 {
		return connectOpts{}, errors.New("--max-body must be positive")
	}
	opts := connectOpts{Relay: *relay, Path: *path, Port: *port, Token: *token, MaxBody: *body}
	set := visited(fs)
	if !set["relay"] {
		opts.Relay = file.Relay
	}
	if !set["path"] {
		opts.Path = file.Path
	}
	if !set["port"] {
		opts.Port = file.Port
	}
	if opts.Relay == "" {
		return opts, errors.New("relay URL is required (--relay or .plumb.toml)")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return opts, errors.New("local port is required (--port or .plumb.toml)")
	}
	return opts, nil
}

func parseReplay(args []string, file fileConfig) (replayOpts, error) {
	maxBody, err := envInt("PLUMB_MAX_BODY", 2<<20)
	if err != nil {
		return replayOpts{}, err
	}
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	last := fs.Int("last", 5, "number of buffered webhooks to re-POST")
	relay := fs.String("relay", "", "relay base URL")
	path := fs.String("path", "", "tunnel path")
	port := fs.Int("port", 0, "local port to forward to")
	token := fs.String("token", os.Getenv("PLUMB_TOKEN"), "tunnel token (or PLUMB_TOKEN)")
	body := fs.Int("max-body", maxBody, "maximum body bytes (or PLUMB_MAX_BODY)")
	if err := fs.Parse(args); err != nil {
		return replayOpts{}, err
	}
	if *body < 1 {
		return replayOpts{}, errors.New("--max-body must be positive")
	}
	opts := replayOpts{
		connectOpts: connectOpts{Relay: *relay, Path: *path, Port: *port, Token: *token, MaxBody: *body},
		Last:        *last,
	}
	set := visited(fs)
	if !set["relay"] {
		opts.Relay = file.Relay
	}
	if !set["path"] {
		opts.Path = file.Path
	}
	if !set["port"] {
		opts.Port = file.Port
	}
	if opts.Last < 1 {
		return opts, errors.New("--last must be positive")
	}
	return opts, nil
}

func parseRelay(args []string) (relayOpts, error) {
	bufferN, err := envInt("PLUMB_BUFFER", 50)
	if err != nil {
		return relayOpts{}, err
	}
	maxBody, err := envInt("PLUMB_MAX_BODY", 2<<20)
	if err != nil {
		return relayOpts{}, err
	}
	timeout, err := envDuration("PLUMB_FORWARD_TIMEOUT", 15*time.Second)
	if err != nil {
		return relayOpts{}, err
	}
	maxPaths, err := envInt("PLUMB_MAX_PATHS", 1024)
	if err != nil {
		return relayOpts{}, err
	}
	pathTTL, err := envDuration("PLUMB_PATH_TTL", 24*time.Hour)
	if err != nil {
		return relayOpts{}, err
	}

	fs := flag.NewFlagSet("relay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "", "listen address (default :$PORT or :8080)")
	token := fs.String("token", os.Getenv("PLUMB_TOKEN"), "shared tunnel token (or PLUMB_TOKEN)")
	dev := fs.Bool("dev", false, "allow tunnels with no token; localhost only")
	buffer := fs.Int("buffer", bufferN, "webhooks retained per path")
	body := fs.Int("max-body", maxBody, "maximum request body in bytes")
	forward := fs.Duration("forward-timeout", timeout, "how long to wait for localhost")
	if err := fs.Parse(args); err != nil {
		return relayOpts{}, err
	}
	if *token == "" && !*dev {
		return relayOpts{}, errors.New("set PLUMB_TOKEN or --token, or pass --dev for localhost")
	}
	if *buffer < 1 || *body < 1 || *forward <= 0 || maxPaths < 1 {
		return relayOpts{}, errors.New("buffer, max-body, forward-timeout, and max paths must be positive")
	}
	return relayOpts{
		Addr:           resolveListenAddr(*addr),
		Token:          *token,
		Dev:            *dev,
		Buffer:         *buffer,
		MaxBody:        *body,
		ForwardTimeout: *forward,
		MaxPaths:       maxPaths,
		PathTTL:        pathTTL,
	}, nil
}

func visited(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `Plumb is a webhook relay for local development.

Usage:
  plumb relay [flags]
  plumb connect [flags]
  plumb replay [flags]
  plumb version

Relay:
  --addr string              listen address (default :$PORT or :8080)
  --token string             shared tunnel token (or PLUMB_TOKEN)
  --dev                      allow tunnels with no token; localhost only
  --buffer int               webhooks retained per path (or PLUMB_BUFFER)
  --max-body int             maximum body bytes (or PLUMB_MAX_BODY)
  --forward-timeout duration wait for localhost (or PLUMB_FORWARD_TIMEOUT)

  PLUMB_MAX_PATHS            maximum tunnel paths (default 1024)
  PLUMB_PATH_TTL             drop an idle path after this duration (default 24h)

Connect:
  --relay string             relay base URL
  --port int                 local port to forward to
  --path string              tunnel path; random when omitted
  --token string             tunnel token (or PLUMB_TOKEN)
  --max-body int             maximum body bytes (or PLUMB_MAX_BODY)

Replay:
  --last int                 buffered webhooks to re-POST (default 5)
  --max-body int             maximum body bytes when connect is not running
  --relay --path --port --token
                             required only when plumb connect is not running

A .plumb.toml file in the working directory can set relay, path, and port.
Flags override the file. The token is never read from the file.
`)
}
