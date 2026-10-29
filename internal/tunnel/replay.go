package tunnel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/rejisterjack/plumb/internal/protocol"
)

// PullConfig is a one-shot replay that does not take over the live tunnel.
type PullConfig struct {
	RelayURL string
	Path     string
	Token    string
	Port     int
	Last     int
	MaxBody  int64
	Out      io.Writer
}

// PullAndDeliver opens a short-lived replay connection, pulls the ring, and
// posts each entry to localhost. It does not register a forwarding tunnel.
func PullAndDeliver(ctx context.Context, cfg PullConfig) (int, error) {
	if cfg.Last <= 0 {
		cfg.Last = 5
	}
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = defaultMaxBody
	}
	if !protocol.ValidSlug(cfg.Path) {
		return 0, errors.New("invalid path")
	}
	wsURL, err := TunnelURL(cfg.RelayURL)
	if err != nil {
		return 0, err
	}
	dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	dialCancel()
	if err != nil {
		return 0, err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(cfg.MaxBody*2 + 1<<20)

	ioCtx, ioCancel := context.WithCancel(ctx)
	defer ioCancel()
	if err := writeMsg(ioCtx, conn, protocol.Message{
		Type:  protocol.TypeRegister,
		Mode:  protocol.ModeReplay,
		Path:  cfg.Path,
		Token: cfg.Token,
		Last:  cfg.Last,
	}); err != nil {
		return 0, err
	}
	for {
		msg, err := readMsg(ioCtx, conn)
		if err != nil {
			return 0, err
		}
		switch msg.Type {
		case protocol.TypeError:
			return 0, errors.New(msg.Error)
		case protocol.TypeReplayBatch:
			return deliverItems(ioCtx, cfg.Port, cfg.MaxBody, cfg.Out, msg.Items, true)
		}
	}
}

func exchange(parent context.Context, port int, maxBody int64, out io.Writer, replay bool, timeout time.Duration, method, urlPath, host, original string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	start := time.Now()
	status, hdr, respBody, err := Forward(ctx, port, maxBody, method, urlPath, host, original, headers, body)
	if err != nil {
		LogExchange(out, replay, method, urlPath, http.StatusBadGateway, time.Since(start), headers, body, err.Error())
		return http.StatusBadGateway, nil, []byte("bad gateway"), err
	}
	LogExchange(out, replay, method, urlPath, status, time.Since(start), headers, body, "")
	return status, hdr, respBody, nil
}

func deliverItems(ctx context.Context, port int, maxBody int64, out io.Writer, items []protocol.Item, replay bool) (int, error) {
	delivered := 0
	var first error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return delivered, err
		}
		body, err := protocol.DecodeBody(item.Body)
		if err != nil {
			return delivered, err
		}
		_, _, _, ferr := exchange(ctx, port, maxBody, out, replay, 15*time.Second, item.Method, item.URL, item.Host, item.OriginalURL, item.Headers, body)
		if ferr != nil {
			if first == nil {
				first = ferr
			}
			continue
		}
		delivered++
	}
	return delivered, first
}
