package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/rejisterjack/plumb/internal/protocol"
)

const (
	defaultBackoffMin  = 500 * time.Millisecond
	defaultBackoffMax  = 30 * time.Second
	defaultPingEvery   = 20 * time.Second
	defaultIdleTimeout = 60 * time.Second
	defaultMaxBody     = 2 << 20
)

// Client maintains a tunnel to a relay and forwards webhooks to localhost.
type Client struct {
	RelayURL     string
	Path         string
	Token        string
	Port         int
	MaxBody      int64
	Out          io.Writer
	Err          io.Writer
	SocketDir    string
	BackoffMin   time.Duration
	BackoffMax   time.Duration
	PingInterval time.Duration
	IdleTimeout  time.Duration

	mu        sync.Mutex
	active    *session
	announced bool
	ln        interface{ Close() error }
	sockPath  string
}

type session struct {
	c      *Client
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	writeMu    sync.Mutex
	mu         sync.Mutex
	replayWait chan protocol.Message
}

// Run connects until ctx is cancelled, reconnecting silently after the first
// successful registration.
func (c *Client) Run(ctx context.Context) error {
	if err := c.prepare(); err != nil {
		return err
	}
	c.Out = &lockedWriter{w: c.out()}
	c.Err = &lockedWriter{w: c.errw()}
	if err := c.serveControl(ctx); err != nil {
		return err
	}
	defer c.closeControl()

	attempt := 0
	announced := false
	var lastErr string
	for {
		if ctx.Err() != nil {
			return nil
		}
		registered, err := c.runOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if registered {
			attempt = 0
			announced = true
			lastErr = ""
		}
		if err != nil && !announced {
			msg := err.Error()
			if msg != lastErr {
				fmt.Fprintf(c.errw(), "plumb: %v\n", err)
				lastErr = msg
			}
		}
		wait := backoff(attempt, c.BackoffMin, c.BackoffMax)
		if !registered {
			attempt++
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// Connected reports whether a tunnel is registered right now.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active != nil
}

func (c *Client) prepare() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if _, err := parseRelay(c.RelayURL); err != nil {
		return err
	}
	if c.Path == "" {
		slug, err := randomSlug()
		if err != nil {
			return err
		}
		c.Path = slug
	}
	if !protocol.ValidSlug(c.Path) {
		return fmt.Errorf("invalid path %q", c.Path)
	}
	if c.MaxBody <= 0 {
		c.MaxBody = defaultMaxBody
	}
	if c.BackoffMin <= 0 {
		c.BackoffMin = defaultBackoffMin
	}
	if c.BackoffMax <= 0 {
		c.BackoffMax = defaultBackoffMax
	}
	if c.BackoffMax < c.BackoffMin {
		c.BackoffMax = c.BackoffMin
	}
	if c.PingInterval <= 0 {
		c.PingInterval = defaultPingEvery
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = defaultIdleTimeout
	}
	return nil
}

func (c *Client) runOnce(ctx context.Context) (registered bool, err error) {
	wsURL, err := TunnelURL(c.RelayURL)
	if err != nil {
		return false, err
	}
	dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	dialCancel()
	if err != nil {
		return false, err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(c.readLimit())

	sessCtx, sessCancel := context.WithCancel(context.Background())
	defer sessCancel()
	go func() {
		select {
		case <-ctx.Done():
			sessCancel()
		case <-sessCtx.Done():
		}
	}()

	sess := &session{c: c, conn: conn, ctx: sessCtx, cancel: sessCancel}
	if err := sess.write(protocol.Message{
		Type:  protocol.TypeRegister,
		Token: c.Token,
		Path:  c.Path,
		Mode:  protocol.ModeTunnel,
	}); err != nil {
		return false, err
	}
	msg, err := readMsg(sessCtx, conn)
	if err != nil {
		return false, err
	}
	if msg.Type == protocol.TypeError {
		return false, errors.New(msg.Error)
	}
	if msg.Type != protocol.TypeRegistered || msg.Path != c.Path {
		return false, fmt.Errorf("unexpected message %q", msg.Type)
	}

	c.setActive(sess)
	defer c.clearActive(sess)
	c.announce()
	return true, sess.readLoop()
}

func (c *Client) announce() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.announced {
		return
	}
	c.announced = true
	pub, err := PublicURL(c.RelayURL, c.Path)
	if err != nil {
		return
	}
	fmt.Fprintf(c.out(), "public: %s\nlocal:  http://127.0.0.1:%d\n", pub, c.Port)
}

func (c *Client) setActive(s *session) {
	c.mu.Lock()
	c.active = s
	c.mu.Unlock()
}

func (c *Client) clearActive(s *session) {
	c.mu.Lock()
	if c.active == s {
		c.active = nil
	}
	c.mu.Unlock()
}

func (c *Client) readLimit() int64 {
	maxBody := c.MaxBody
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	return maxBody*2 + 1<<20
}

func (c *Client) out() io.Writer {
	if c.Out != nil {
		return c.Out
	}
	return os.Stdout
}

func (c *Client) errw() io.Writer {
	if c.Err != nil {
		return c.Err
	}
	return os.Stderr
}

// lockedWriter serializes terminal output from concurrent forwards.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (s *session) readLoop() error {
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	stop := make(chan struct{})
	defer close(stop)
	go s.c.watchIdle(s, stop, &last)
	go s.ping()

	for {
		msg, err := readMsg(s.ctx, s.conn)
		if err != nil {
			return err
		}
		last.Store(time.Now().UnixNano())
		switch msg.Type {
		case protocol.TypeRequest:
			go s.handleRequest(msg)
		case protocol.TypeReplayBatch:
			s.deliverReplay(msg)
		case protocol.TypePing:
			_ = s.write(protocol.Message{Type: protocol.TypePong})
		case protocol.TypePong:
		case protocol.TypeError:
			return errors.New(msg.Error)
		}
	}
}

func (s *session) ping() {
	ticker := time.NewTicker(s.c.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.write(protocol.Message{Type: protocol.TypePing}); err != nil {
				s.cancel()
				return
			}
		}
	}
}

func (c *Client) watchIdle(s *session, stop <-chan struct{}, last *atomic.Int64) {
	period := c.IdleTimeout / 3
	if period < 50*time.Millisecond {
		period = 50 * time.Millisecond
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, last.Load())) > c.IdleTimeout {
				s.cancel()
				return
			}
		}
	}
}

func (s *session) handleRequest(msg protocol.Message) {
	body, err := protocol.DecodeBody(msg.Body)
	if err != nil {
		_ = s.write(protocol.Message{
			Type:   protocol.TypeResponse,
			ID:     msg.ID,
			Status: http.StatusBadRequest,
			Body:   protocol.EncodeBody([]byte("bad body")),
		})
		return
	}
	timeout := time.Duration(msg.TimeoutMs) * time.Millisecond
	status, hdr, respBody, _ := exchange(s.ctx, s.c.Port, s.c.MaxBody, s.c.out(), false, timeout, msg.Method, msg.URL, msg.Host, msg.OriginalURL, msg.Headers, body)
	_ = s.write(protocol.Message{
		Type:    protocol.TypeResponse,
		ID:      msg.ID,
		Status:  status,
		Headers: hdr,
		Body:    protocol.EncodeBody(respBody),
	})
}

func (s *session) write(msg protocol.Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeMsg(s.ctx, s.conn, msg)
}

func (s *session) deliverReplay(msg protocol.Message) {
	s.mu.Lock()
	ch := s.replayWait
	s.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

func (s *session) fetchReplay(ctx context.Context, last int) ([]protocol.Item, error) {
	ch := make(chan protocol.Message, 1)
	s.mu.Lock()
	if s.replayWait != nil {
		s.mu.Unlock()
		return nil, errors.New("replay already in progress")
	}
	s.replayWait = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.replayWait = nil
		s.mu.Unlock()
	}()
	if err := s.write(protocol.Message{Type: protocol.TypeReplay, Last: last}); err != nil {
		return nil, err
	}
	select {
	case msg := <-ch:
		if msg.Type == protocol.TypeError {
			return nil, errors.New(msg.Error)
		}
		return msg.Items, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("tunnel closed")
	}
}

// DeliverLast asks the relay for the newest n buffered webhooks and posts
// them to localhost again.
func (c *Client) DeliverLast(ctx context.Context, last int) (int, error) {
	if last <= 0 {
		last = 5
	}
	c.mu.Lock()
	s := c.active
	c.mu.Unlock()
	if s == nil {
		return 0, errors.New("not connected")
	}
	items, err := s.fetchReplay(ctx, last)
	if err != nil {
		return 0, err
	}
	return deliverItems(ctx, c.Port, c.MaxBody, c.out(), items, true)
}

func readMsg(ctx context.Context, conn *websocket.Conn) (protocol.Message, error) {
	_, b, err := conn.Read(ctx)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Unmarshal(b)
}

func writeMsg(ctx context.Context, conn *websocket.Conn, msg protocol.Message) error {
	b, err := protocol.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, b)
}

func backoff(attempt int, min, max time.Duration) time.Duration {
	if min <= 0 {
		min = defaultBackoffMin
	}
	if max < min {
		max = min
	}
	if attempt < 0 {
		attempt = 0
	}
	d := min
	for i := 0; i < attempt; i++ {
		if d > max/2 {
			d = max
			break
		}
		d *= 2
		if d > max {
			d = max
			break
		}
	}
	span := d / 4
	if span <= 0 {
		return d
	}
	n := rand.Int64N(int64(span) + 1)
	jittered := d - span/2 + time.Duration(n)
	if jittered > max {
		return max
	}
	floor := min / 2
	if floor <= 0 {
		floor = min
	}
	if jittered < floor {
		return floor
	}
	return jittered
}
