package relay

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/rejisterjack/plumb/internal/buffer"
	"github.com/rejisterjack/plumb/internal/protocol"
)

type pathState struct {
	name     string
	buf      *buffer.Ring
	inflight chan struct{}

	mu            sync.Mutex
	conn          *websocket.Conn
	writeMu       sync.Mutex
	sessionCtx    context.Context
	sessionCancel context.CancelFunc
	pending       map[string]chan protocol.Message
}

func newPathState(cfg Config) *pathState {
	return &pathState{
		buf:      buffer.New(cfg.BufferSize),
		inflight: make(chan struct{}, cfg.MaxInFlight),
	}
}

func (s *Server) tunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(s.cfg.MaxBody*2 + 1<<20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg, err := s.readRegister(ctx, conn)
	if err != nil {
		return
	}
	if !s.authorized(msg.Token) {
		_ = writeMsg(ctx, conn, nil, protocol.Message{Type: protocol.TypeError, Error: "unauthorized"})
		return
	}
	if !protocol.ValidSlug(msg.Path) {
		_ = writeMsg(ctx, conn, nil, protocol.Message{Type: protocol.TypeError, Error: "invalid path"})
		return
	}
	mode := msg.Mode
	if mode == "" {
		mode = protocol.ModeTunnel
	}
	if mode == protocol.ModeReplay {
		_ = s.writeReplay(ctx, conn, nil, msg.Path, msg.Last)
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return
	}
	if mode != protocol.ModeTunnel {
		_ = writeMsg(ctx, conn, nil, protocol.Message{Type: protocol.TypeError, Error: "invalid mode"})
		return
	}

	ps, err := s.claim(msg.Path, conn, ctx, cancel)
	if err != nil {
		_ = writeMsg(ctx, conn, nil, protocol.Message{Type: protocol.TypeError, Error: "path in use"})
		return
	}
	defer s.unclaim(msg.Path, conn)

	if err := ps.write(protocol.Message{Type: protocol.TypeRegistered, Path: msg.Path}); err != nil {
		return
	}
	s.readLoop(ps)
}

func (s *Server) readRegister(ctx context.Context, conn *websocket.Conn) (protocol.Message, error) {
	// CompareAndSwap so the timeout and a successful read cannot both decide
	// the outcome. Canceling the read context would close the socket, so the
	// timeout closes the socket only if it wins the race.
	var done atomic.Bool
	timer := time.AfterFunc(s.cfg.RegisterTimeout, func() {
		if done.CompareAndSwap(false, true) {
			_ = conn.Close(websocket.StatusPolicyViolation, "register timeout")
		}
	})
	defer timer.Stop()
	msg, err := readMsg(ctx, conn)
	if !done.CompareAndSwap(false, true) {
		return protocol.Message{}, errTunnel
	}
	if err != nil {
		return protocol.Message{}, err
	}
	if msg.Type != protocol.TypeRegister {
		_ = writeMsg(ctx, conn, nil, protocol.Message{Type: protocol.TypeError, Error: "expected register"})
		return protocol.Message{}, errTunnel
	}
	return msg, nil
}

func (s *Server) claim(path string, conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc) (*pathState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.paths[path]
	if ps == nil {
		ps = newPathState(s.cfg)
		s.paths[path] = ps
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.conn != nil {
		return nil, errPathInUse
	}
	ps.name = path
	ps.conn = conn
	ps.sessionCtx = ctx
	ps.sessionCancel = cancel
	ps.pending = make(map[string]chan protocol.Message)
	return ps, nil
}

func (s *Server) unclaim(path string, conn *websocket.Conn) {
	ps := s.pathState(path)
	if ps == nil {
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.conn != conn {
		return
	}
	ps.conn = nil
	if ps.sessionCancel != nil {
		ps.sessionCancel()
		ps.sessionCancel = nil
	}
	ps.sessionCtx = nil
	ps.pending = nil
}

// disconnect cancels the active tunnel for path. Tests use it to simulate a drop.
func (s *Server) disconnect(path string) {
	ps := s.pathState(path)
	if ps == nil {
		return
	}
	ps.mu.Lock()
	cancel := ps.sessionCancel
	ps.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) readLoop(ps *pathState) {
	ps.mu.Lock()
	conn := ps.conn
	ctx := ps.sessionCtx
	ps.mu.Unlock()
	if conn == nil || ctx == nil {
		return
	}

	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	stop := make(chan struct{})
	defer close(stop)
	go s.watchIdle(ctx, stop, &last, ps)

	for {
		msg, err := readMsg(ctx, conn)
		if err != nil {
			return
		}
		last.Store(time.Now().UnixNano())
		switch msg.Type {
		case protocol.TypeResponse:
			ps.deliver(msg)
		case protocol.TypePing:
			_ = ps.write(protocol.Message{Type: protocol.TypePong})
		case protocol.TypePong:
		case protocol.TypeReplay:
			_ = s.writeReplay(ctx, conn, &ps.writeMu, ps.name, msg.Last)
		default:
		}
	}
}

func (s *Server) watchIdle(ctx context.Context, stop <-chan struct{}, last *atomic.Int64, ps *pathState) {
	period := s.cfg.IdleTimeout / 3
	if period < 50*time.Millisecond {
		period = 50 * time.Millisecond
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, last.Load())) > s.cfg.IdleTimeout {
				ps.mu.Lock()
				cancel := ps.sessionCancel
				ps.mu.Unlock()
				if cancel != nil {
					cancel()
				}
				return
			}
		}
	}
}

func (s *Server) forward(ps *pathState, e buffer.Entry) (int, http.Header, []byte, error) {
	select {
	case ps.inflight <- struct{}{}:
		defer func() { <-ps.inflight }()
	default:
		return http.StatusTooManyRequests, nil, nil, errBusy
	}

	ch := make(chan protocol.Message, 1)
	ps.mu.Lock()
	if ps.conn == nil || ps.sessionCtx == nil || ps.pending == nil {
		ps.mu.Unlock()
		return http.StatusOK, nil, nil, errOffline
	}
	sessionCtx := ps.sessionCtx
	ps.pending[e.ID] = ch
	ps.mu.Unlock()
	defer func() {
		ps.mu.Lock()
		delete(ps.pending, e.ID)
		ps.mu.Unlock()
	}()

	err := ps.write(protocol.Message{
		Type:        protocol.TypeRequest,
		ID:          e.ID,
		Method:      e.Method,
		URL:         e.URL,
		Host:        e.Host,
		OriginalURL: e.OriginalURL,
		Headers:     e.Headers,
		Body:        protocol.EncodeBody(e.Body),
		TimeoutMs:   s.cfg.ForwardTimeout.Milliseconds(),
	})
	if err != nil {
		return http.StatusBadGateway, nil, nil, err
	}

	timer := time.NewTimer(s.cfg.ForwardTimeout)
	defer timer.Stop()
	select {
	case resp := <-ch:
		body, decErr := protocol.DecodeBody(resp.Body)
		if decErr != nil {
			return http.StatusBadGateway, nil, nil, decErr
		}
		if int64(len(body)) > s.cfg.MaxBody {
			return http.StatusBadGateway, nil, nil, errTooLarge
		}
		status := resp.Status
		if status < 100 || status > 599 {
			status = http.StatusBadGateway
		}
		return status, resp.Headers, body, nil
	case <-timer.C:
		return http.StatusGatewayTimeout, nil, nil, errTimeout
	case <-sessionCtx.Done():
		return http.StatusBadGateway, nil, nil, errTunnel
	}
}

func (ps *pathState) write(msg protocol.Message) error {
	ps.mu.Lock()
	conn := ps.conn
	ctx := ps.sessionCtx
	ps.mu.Unlock()
	if conn == nil || ctx == nil {
		return errTunnel
	}
	return writeMsg(ctx, conn, &ps.writeMu, msg)
}

func (ps *pathState) deliver(msg protocol.Message) {
	ps.mu.Lock()
	ch := ps.pending[msg.ID]
	ps.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

func (s *Server) writeReplay(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, path string, last int) error {
	if last <= 0 {
		last = 5
	}
	var items []protocol.Item
	if ps := s.pathState(path); ps != nil {
		for _, e := range ps.buf.Last(last) {
			items = append(items, itemFrom(e))
		}
	}
	return writeMsg(ctx, conn, mu, protocol.Message{Type: protocol.TypeReplayBatch, Items: items})
}

func itemFrom(e buffer.Entry) protocol.Item {
	return protocol.Item{
		ID:          e.ID,
		Method:      e.Method,
		URL:         e.URL,
		Host:        e.Host,
		OriginalURL: e.OriginalURL,
		Headers:     e.Headers.Clone(),
		Body:        protocol.EncodeBody(e.Body),
		Time:        e.Time,
	}
}

func readMsg(ctx context.Context, conn *websocket.Conn) (protocol.Message, error) {
	_, b, err := conn.Read(ctx)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Unmarshal(b)
}

func writeMsg(ctx context.Context, conn *websocket.Conn, mu *sync.Mutex, msg protocol.Message) error {
	b, err := protocol.Marshal(msg)
	if err != nil {
		return err
	}
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	return conn.Write(ctx, websocket.MessageText, b)
}
