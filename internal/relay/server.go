package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rejisterjack/plumb/internal/buffer"
	"github.com/rejisterjack/plumb/internal/httpheader"
)

const (
	defaultBufferSize     = 50
	defaultMaxBody        = 2 << 20
	defaultForwardTimeout = 15 * time.Second
	defaultMaxInFlight    = 32
	defaultRatePerSec     = 100
	defaultRateBurst      = 200
	defaultIdleTimeout    = 60 * time.Second
	defaultRegisterWait   = 10 * time.Second
	defaultMaxPaths       = 1024
	defaultPathTTL        = 24 * time.Hour
)

var (
	errOffline      = errors.New("tunnel offline")
	errBusy         = errors.New("too many in-flight forwards")
	errTimeout      = errors.New("forward timeout")
	errTunnel       = errors.New("tunnel closed")
	errTooLarge     = errors.New("body too large")
	errPathInUse    = errors.New("path in use")
	errTooManyPaths = errors.New("too many paths")
)

// Config controls a relay process. Zero values receive the defaults except
// Token, which is required unless Dev is set.
type Config struct {
	Token           string
	Dev             bool
	BufferSize      int
	MaxBody         int64
	ForwardTimeout  time.Duration
	MaxInFlight     int
	RatePerSec      float64
	RateBurst       float64
	IdleTimeout     time.Duration
	RegisterTimeout time.Duration
	MaxPaths        int
	PathTTL         time.Duration
}

func (c Config) withDefaults() Config {
	if c.BufferSize <= 0 {
		c.BufferSize = defaultBufferSize
	}
	if c.MaxBody <= 0 {
		c.MaxBody = defaultMaxBody
	}
	if c.ForwardTimeout <= 0 {
		c.ForwardTimeout = defaultForwardTimeout
	}
	if c.MaxInFlight <= 0 {
		c.MaxInFlight = defaultMaxInFlight
	}
	if c.RateBurst <= 0 && c.RatePerSec <= 0 {
		c.RateBurst = defaultRateBurst
		c.RatePerSec = defaultRatePerSec
	}
	if c.RateBurst <= 0 {
		c.RateBurst = 1
	}
	if c.RatePerSec < 0 {
		c.RatePerSec = 0
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = defaultIdleTimeout
	}
	if c.RegisterTimeout <= 0 {
		c.RegisterTimeout = defaultRegisterWait
	}
	if c.MaxPaths <= 0 {
		c.MaxPaths = defaultMaxPaths
	}
	if c.PathTTL <= 0 {
		c.PathTTL = defaultPathTTL
	}
	return c
}

// Server is the webhook ingress and WebSocket tunnel endpoint.
type Server struct {
	cfg     Config
	log     *slog.Logger
	limiter *limiter

	mu        sync.Mutex
	paths     map[string]*pathState
	stop      chan struct{}
	closeOnce sync.Once
}

// New builds a relay handler. log may be nil.
func New(cfg Config, log *slog.Logger) *Server {
	cfg = cfg.withDefaults()
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := &Server{
		cfg:     cfg,
		log:     log,
		limiter: newLimiter(cfg.RatePerSec, cfg.RateBurst),
		paths:   make(map[string]*pathState),
		stop:    make(chan struct{}),
	}
	go s.sweep()
	return s
}

// Close stops path expiry and drops every tunnel. It is safe to call twice.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.CloseTunnels()
	})
}

// HTTPServer is the process-facing server. WriteTimeout covers a body read
// plus one forward. Hijacked tunnel sockets clear that deadline.
func HTTPServer(addr string, handler http.Handler, forwardTimeout time.Duration) *http.Server {
	if forwardTimeout <= 0 {
		forwardTimeout = defaultForwardTimeout
	}
	const readTimeout = 60 * time.Second
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      readTimeout + forwardTimeout + 15*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

// Handler serves /healthz, /tunnel, and /h/{path}/... webhook routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/tunnel", s.tunnel)
	mux.HandleFunc("/", s.ingress)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject this before ServeMux turns "/../" into a redirect. A trailing
		// slash on an otherwise clean path is kept so the public prefix works.
		if uncleanRequestPath(r.URL.Path) {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func uncleanRequestPath(p string) bool {
	if p == "" || strings.Contains(p, "\\") {
		return true
	}
	cleaned := path.Clean(p)
	if cleaned == p {
		return false
	}
	return cleaned+"/" != p
}

// CloseTunnels drops every connected CLI so in-flight forwards unblock.
func (s *Server) CloseTunnels() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.paths))
	for _, ps := range s.paths {
		ps.mu.Lock()
		if ps.sessionCancel != nil {
			cancels = append(cancels, ps.sessionCancel)
		}
		ps.mu.Unlock()
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (s *Server) ingress(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	slug, rest, ok := SplitHookPath(r.URL.RequestURI())
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.limiter.Allow(clientIP(r)) {
		s.finish(w, slug, r.Method, http.StatusTooManyRequests, 0, start, nil, nil)
		return
	}
	if !allowedMethod(r.Method) {
		s.finish(w, slug, r.Method, http.StatusMethodNotAllowed, 0, start, nil, nil)
		return
	}
	ps := s.pathState(slug)
	if ps == nil {
		http.NotFound(w, r)
		s.logReq(slug, r.Method, http.StatusNotFound, 0, time.Since(start))
		return
	}
	body, err := readBody(w, r.Body, s.cfg.MaxBody)
	if errors.Is(err, errTooLarge) {
		s.finish(w, slug, r.Method, http.StatusRequestEntityTooLarge, 0, start, nil, nil)
		return
	}
	if err != nil {
		s.finish(w, slug, r.Method, http.StatusBadRequest, 0, start, nil, nil)
		return
	}
	id, err := newID()
	if err != nil {
		s.finish(w, slug, r.Method, http.StatusInternalServerError, len(body), start, nil, nil)
		return
	}
	entry := buffer.Entry{
		ID:          id,
		Method:      r.Method,
		URL:         rest,
		Host:        r.Host,
		OriginalURL: originalURL(r),
		Headers:     r.Header.Clone(),
		Body:        body,
		Time:        time.Now().UTC(),
	}
	ps.buf.Add(entry)

	ps.mu.Lock()
	ps.lastActive = time.Now()
	online := ps.conn != nil
	ps.mu.Unlock()
	if !online {
		hdr := http.Header{"X-Plumb-Buffered": []string{"1"}}
		s.finish(w, slug, r.Method, http.StatusOK, len(body), start, hdr, nil)
		return
	}

	status, hdr, respBody, ferr := s.forward(ps, entry)
	if errors.Is(ferr, errOffline) || errors.Is(ferr, errTunnel) {
		hdr = http.Header{"X-Plumb-Buffered": []string{"1"}}
		s.finish(w, slug, r.Method, http.StatusOK, len(body), start, hdr, nil)
		return
	}
	if ferr != nil {
		s.finish(w, slug, r.Method, status, len(body), start, nil, nil)
		return
	}
	s.finish(w, slug, r.Method, status, len(body), start, hdr, respBody)
}

func (s *Server) finish(w http.ResponseWriter, path, method string, status, n int, start time.Time, hdr http.Header, body []byte) {
	s.logReq(path, method, status, n, time.Since(start))
	// A nil body is an error this process generated. An empty slice is a real
	// upstream response, and its status and headers must be preserved.
	if status >= 400 && body == nil {
		http.Error(w, http.StatusText(status), status)
		return
	}
	dst := w.Header()
	copied := httpheader.CloneForward(hdr)
	for k, vs := range copied {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	if hdr != nil {
		if v := hdr.Get("X-Plumb-Buffered"); v != "" {
			dst.Set("X-Plumb-Buffered", v)
		}
	}
	if status < 100 || status > 599 {
		status = http.StatusBadGateway
	}
	w.WriteHeader(status)
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
}

func (s *Server) logReq(path, method string, status, n int, dur time.Duration) {
	s.log.Info("webhook",
		"path", path,
		"method", method,
		"status", status,
		"bytes", n,
		"duration", dur,
	)
}

func (s *Server) authorized(token string) bool {
	if s.cfg.Token == "" {
		return s.cfg.Dev
	}
	return tokenMatch(token, s.cfg.Token)
}

func (s *Server) pathState(slug string) *pathState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paths[slug]
}

func readBody(w http.ResponseWriter, body io.ReadCloser, max int64) ([]byte, error) {
	defer body.Close()
	b, err := io.ReadAll(http.MaxBytesReader(w, body, max))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errTooLarge
		}
		return nil, err
	}
	return b, nil
}

func allowedMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	// RemoteAddr is the TCP peer. X-Forwarded-For is client-controlled unless
	// every proxy hop is trusted, so a spoofed header must not mint a fresh
	// rate-limit bucket. Behind Render or Railway the peer is the platform
	// proxy and the limit is effectively global, which is what a single-team
	// relay wants.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func originalURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "https" || p == "http" {
		scheme = p
	}
	return scheme + "://" + r.Host + r.URL.RequestURI()
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
