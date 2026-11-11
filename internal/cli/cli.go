package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rejisterjack/plumb/internal/relay"
	"github.com/rejisterjack/plumb/internal/tunnel"
)

// Main runs the plumb command and returns an exit code.
func Main(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintf(os.Stdout, "plumb %s\n", version)
		return 0
	case "relay":
		return runRelay(args[1:])
	case "connect":
		return runConnect(args[1:])
	case "replay":
		return runReplay(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "plumb: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func runRelay(args []string) int {
	opts, err := parseRelay(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 2
	}
	if opts.Dev && opts.Token == "" {
		fmt.Fprintf(os.Stderr, "plumb relay: dev mode, tunnel authentication disabled\n")
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := relay.New(relay.Config{
		Token:          opts.Token,
		Dev:            opts.Dev,
		BufferSize:     opts.Buffer,
		MaxBody:        int64(opts.MaxBody),
		ForwardTimeout: opts.ForwardTimeout,
		MaxPaths:       opts.MaxPaths,
		PathTTL:        opts.PathTTL,
	}, log)
	httpSrv := relay.HTTPServer(opts.Addr, srv.Handler(), opts.ForwardTimeout)
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()
	fmt.Fprintf(os.Stderr, "plumb relay listening on %s\n", opts.Addr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 1
	case <-ctx.Done():
	}

	srv.Close()
	shutCtx, cancel := context.WithTimeout(context.Background(), opts.ForwardTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 1
	}
	return 0
}

func runConnect(args []string) int {
	file, err := loadDefaultConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 2
	}
	opts, err := parseConnect(args, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	client := &tunnel.Client{
		RelayURL: opts.Relay,
		Path:     opts.Path,
		Token:    opts.Token,
		Port:     opts.Port,
		MaxBody:  int64(opts.MaxBody),
	}
	if err := client.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 1
	}
	return 0
}

func runReplay(args []string) int {
	file, err := loadDefaultConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 2
	}
	opts, err := parseReplay(args, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 2
	}
	sock, err := tunnel.RunningSocket("", opts.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 1
	}
	var n int
	if sock != "" {
		n, err = tunnel.ReplayLocal(sock, opts.Last)
	} else {
		if opts.Relay == "" || opts.Path == "" || opts.Port < 1 {
			fmt.Fprintf(os.Stderr, "plumb: connect is not running; pass --relay, --path, and --port\n")
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Last)*20*time.Second+15*time.Second)
		defer cancel()
		n, err = tunnel.PullAndDeliver(ctx, tunnel.PullConfig{
			RelayURL: opts.Relay,
			Path:     opts.Path,
			Token:    opts.Token,
			Port:     opts.Port,
			Last:     opts.Last,
			MaxBody:  int64(opts.MaxBody),
			Out:      os.Stdout,
		})
	}
	if n > 0 || err == nil {
		fmt.Printf("replayed %d\n", n)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		return 1
	}
	return 0
}
