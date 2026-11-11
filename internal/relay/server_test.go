package relay

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rejisterjack/plumb/internal/protocol"
	"github.com/rejisterjack/plumb/internal/tunnel"
)

func TestHealthAndUnknownPath(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret"})
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("ok")) {
		t.Fatalf("health = %d %q", resp.StatusCode, body)
	}
	resp, err = http.Post(ts.URL+"/h/missing/hook", "text/plain", strings.NewReader("nope"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path = %d", resp.StatusCode)
	}
}

func TestHealthHead(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret"})
	req, err := http.NewRequest(http.MethodHead, ts.URL+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD status %d", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("HEAD body = %q", body)
	}
}

func TestTraversalOverHTTP(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.RequestURI())
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	addr := ts.Listener.Addr().String()
	for _, target := range []string{"/h/dev/../../secret", "/h/dev/%2e%2e/secret"} {
		code, statusLine := rawStatus(t, addr, target)
		if code != http.StatusBadRequest {
			t.Fatalf("%s status %d (%s)", target, code, statusLine)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hits) != 0 {
		t.Fatalf("localhost saw %#v", hits)
	}
}

func rawStatus(t *testing.T, addr, target string) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	req := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: %s\r\nContent-Length: 1\r\nConnection: close\r\n\r\nx", target, addr)
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 {
		t.Fatalf("status line %q", line)
	}
	code, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatal(err)
	}
	return code, strings.TrimSpace(line)
}

func TestProxyRawBodyAndResponse(t *testing.T) {
	const payload = `{"id":"evt_1","sig":"a+b/c=="}`
	const sig = "t=1,v1=abc"
	var got struct {
		sync.Mutex
		body, path, query, sig, host, original string
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.Lock()
		got.body = string(b)
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		got.sig = r.Header.Get("Stripe-Signature")
		got.host = r.Host
		got.original = r.Header.Get("X-Plumb-Original-Url")
		got.Unlock()
		if r.Header.Get("X-Forwarded-Host") == "" {
			t.Errorf("missing X-Forwarded-Host")
		}
		w.Header().Set("X-App", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer backend.Close()

	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	log := &memLog{}
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), log)
	waitUntil(t, client.Connected)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/h/dev/webhooks/stripe?x=1", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Stripe-Signature", sig)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || string(respBody) != "created" || resp.Header.Get("X-App") != "yes" {
		t.Fatalf("provider response = %d %q x-app=%q", resp.StatusCode, respBody, resp.Header.Get("X-App"))
	}
	got.Lock()
	defer got.Unlock()
	if got.body != payload || got.sig != sig || got.path != "/webhooks/stripe" || got.query != "x=1" {
		t.Fatalf("local request body=%q sig=%q path=%q query=%q", got.body, got.sig, got.path, got.query)
	}
	if !strings.HasPrefix(got.host, "127.0.0.1:") {
		t.Fatalf("local host = %q", got.host)
	}
	if !strings.Contains(got.original, "/h/dev/webhooks/stripe?x=1") {
		t.Fatalf("original = %q", got.original)
	}
	logged := log.String()
	if !strings.Contains(logged, "POST /webhooks/stripe?x=1 -> 201") || !strings.Contains(logged, payload) || !strings.Contains(logged, sig) {
		t.Fatalf("terminal log = %s", logged)
	}
	if !strings.Contains(logged, "public: "+ts.URL+"/h/dev/") {
		t.Fatalf("missing stable URL in log: %s", logged)
	}
	info, err := os.Stat(clientSocket(t, client))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
}

func TestPathsAreIsolated(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	var mu sync.Mutex
	seen := map[string][]string{}
	newBackend := func(name string) int {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			seen[name] = append(seen[name], string(b))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(srv.Close)
		return portOf(t, srv.URL)
	}
	clientA := startClient(t, ts.URL, "alice", "secret", newBackend("alice"), &memLog{})
	clientB := startClient(t, ts.URL, "bob", "secret", newBackend("bob"), &memLog{})
	waitUntil(t, func() bool { return clientA.Connected() && clientB.Connected() })

	post := func(path, body string) {
		t.Helper()
		resp, err := http.Post(ts.URL+path, "text/plain", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("%s status %d", path, resp.StatusCode)
		}
	}
	post("/h/alice/hook", "for-alice")
	post("/h/bob/hook", "for-bob")

	mu.Lock()
	defer mu.Unlock()
	if len(seen["alice"]) != 1 || seen["alice"][0] != "for-alice" || len(seen["bob"]) != 1 || seen["bob"][0] != "for-bob" {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestBufferWhenTunnelDownAndReplay(t *testing.T) {
	const payload = `{"missed":true}`
	var mu sync.Mutex
	var got []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	srv, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, conn); msg.Type != protocol.TypeRegistered {
		t.Fatalf("register = %+v", msg)
	}
	conn.CloseNow()
	waitUntil(t, func() bool { return !tunnelUp(srv, "dev") })

	resp, err := http.Post(ts.URL+"/h/dev/hook", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Plumb-Buffered") != "1" {
		t.Fatalf("buffered response = %d header %q", resp.StatusCode, resp.Header.Get("X-Plumb-Buffered"))
	}
	if n := srv.pathState("dev").buf.Len(); n != 1 {
		t.Fatalf("buffer len = %d", n)
	}

	n, err := tunnel.PullAndDeliver(ctx, tunnel.PullConfig{
		RelayURL: ts.URL,
		Path:     "dev",
		Token:    "secret",
		Port:     portOf(t, backend.URL),
		Last:     5,
		Out:      io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("replayed %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != payload {
		t.Fatalf("replayed body = %#v", got)
	}
	if tunnelUp(srv, "dev") {
		t.Fatal("replay connection took over the tunnel")
	}
}

func TestReplayDoesNotStealLiveTunnel(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	var mu sync.Mutex
	var got []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer backend.Close()
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("live"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("live status %d", resp.StatusCode)
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer pullCancel()
	n, err := tunnel.PullAndDeliver(pullCtx, tunnel.PullConfig{
		RelayURL: ts.URL,
		Path:     "dev",
		Token:    "secret",
		Port:     portOf(t, backend.URL),
		Last:     1,
		Out:      io.Discard,
	})
	if err != nil || n != 1 {
		t.Fatalf("pull = %d %v", n, err)
	}
	if !client.Connected() {
		t.Fatal("live tunnel dropped during replay")
	}
	resp, err = http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("after"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("after status %d", resp.StatusCode)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "live,live,after" {
		t.Fatalf("bodies = %#v", got)
	}
}

func TestControlSocketReplay(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	var mu sync.Mutex
	var got int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("once"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	n, err := tunnel.ReplayLocal(clientSocket(t, client), 5)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("delivered %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != 2 {
		t.Fatalf("local hits = %d", got)
	}
}

func TestSecondRegisterRejected(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret"})
	ctx := testCtx(t)
	first := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, first, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, first); msg.Type != protocol.TypeRegistered {
		t.Fatalf("first = %+v", msg)
	}
	second := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, second, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	msg := wsRead(t, ctx, second)
	if msg.Type != protocol.TypeError || msg.Error != "path in use" {
		t.Fatalf("second = %+v", msg)
	}
	if !tunnelUp(srv, "dev") {
		t.Fatal("first tunnel dropped")
	}
}

func TestTokenMismatch(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "expected"})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "super-secret-value", Path: "dev"})
	msg := wsRead(t, ctx, conn)
	if msg.Type != protocol.TypeError || msg.Error != "unauthorized" || strings.Contains(msg.Error, "super-secret-value") {
		t.Fatalf("auth = %+v", msg)
	}
	if srv.pathState("dev") != nil {
		t.Fatal("failed auth created a path")
	}
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}

	open := newRelayOnly(t, Config{Token: "", Dev: false})
	ts2 := httptest.NewServer(open.Handler())
	t.Cleanup(ts2.Close)
	conn = dialTunnel(t, ctx, ts2.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "", Path: "dev"})
	msg = wsRead(t, ctx, conn)
	if msg.Type != protocol.TypeError {
		t.Fatalf("empty token = %+v", msg)
	}
}

func TestDevAllowsEmptyToken(t *testing.T) {
	_, ts := newRelay(t, Config{Dev: true, ForwardTimeout: 2 * time.Second})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()
	client := startClient(t, ts.URL, "dev", "", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestBodyTooLarge(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", MaxBody: 8})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, conn); msg.Type != protocol.TypeRegistered {
		t.Fatalf("register = %+v", msg)
	}
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if n := srv.pathState("dev").buf.Len(); n != 0 {
		t.Fatalf("buffer len = %d", n)
	}
}

func TestRateLimit(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret", RateBurst: 1, RatePerSec: 0, ForwardTimeout: 2 * time.Second})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("one"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first status %d", resp.StatusCode)
	}
	resp, err = http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("two"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status %d", resp.StatusCode)
	}
}

func TestForwardTimeout(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 200 * time.Millisecond})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, conn); msg.Type != protocol.TypeRegistered {
		t.Fatalf("register = %+v", msg)
	}
	go func() {
		_, _, _ = conn.Read(ctx)
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("slow"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if srv.pathState("dev").buf.Len() != 1 {
		t.Fatal("timed out request was not buffered")
	}
}

func TestInFlightLimit(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	_, ts := newRelay(t, Config{Token: "secret", MaxInFlight: 1, ForwardTimeout: 3 * time.Second})
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	firstDone := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("hold"))
		if err != nil {
			firstDone <- nil
			return
		}
		firstDone <- resp
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first request did not reach localhost")
	}
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("extra"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status %d", resp.StatusCode)
	}
	close(release)
	select {
	case resp := <-firstDone:
		if resp == nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("first request failed")
		}
		resp.Body.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("first request did not finish")
	}
}

func TestClientReconnectsAndReplays(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	var mu sync.Mutex
	var got []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	log := &memLog{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	client := &tunnel.Client{
		RelayURL:     ts.URL,
		Path:         "dev",
		Token:        "secret",
		Port:         portOf(t, backend.URL),
		Out:          log,
		Err:          io.Discard,
		SocketDir:    shortDir(t),
		PingInterval: time.Hour,
		BackoffMin:   300 * time.Millisecond,
		BackoffMax:   300 * time.Millisecond,
	}
	go func() { _ = client.Run(ctx) }()
	waitUntil(t, client.Connected)

	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("one"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("live status %d", resp.StatusCode)
	}

	srv.disconnect("dev")
	waitUntil(t, func() bool { return !client.Connected() })
	resp, err = http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("two"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Plumb-Buffered") != "1" {
		t.Fatalf("gap response = %d buffered %q", resp.StatusCode, resp.Header.Get("X-Plumb-Buffered"))
	}
	waitUntil(t, client.Connected)

	n, err := tunnel.ReplayLocal(clientSocket(t, client), 5)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("replayed %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "one,one,two" {
		t.Fatalf("bodies = %#v", got)
	}
	if strings.Count(log.String(), "public:") != 1 {
		t.Fatalf("URL announced more than once:\n%s", log.String())
	}
}

func TestLocalErrorStatusIsPreserved(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-App", "no")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer backend.Close()
	_, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("nope"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || len(body) != 0 || resp.Header.Get("X-App") != "no" {
		t.Fatalf("status %d body %q x-app %q", resp.StatusCode, body, resp.Header.Get("X-App"))
	}
}

func TestTunnelDropDuringForwardIsBuffered(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", ForwardTimeout: 2 * time.Second})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, conn); msg.Type != protocol.TypeRegistered {
		t.Fatalf("register = %+v", msg)
	}
	go func() {
		_, _, _ = conn.Read(ctx)
		srv.disconnect("dev")
	}()
	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("mid"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Plumb-Buffered") != "1" {
		t.Fatalf("status %d buffered %q", resp.StatusCode, resp.Header.Get("X-Plumb-Buffered"))
	}
	if srv.pathState("dev").buf.Len() != 1 {
		t.Fatal("request was not buffered")
	}
}

func TestRateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	_, ts := newRelay(t, Config{Token: "secret", RateBurst: 1, RatePerSec: 0, ForwardTimeout: 2 * time.Second})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()
	client := startClient(t, ts.URL, "dev", "secret", portOf(t, backend.URL), &memLog{})
	waitUntil(t, client.Connected)

	post := func(xff string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/h/dev/hook", strings.NewReader("x"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", xff)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if post("1.1.1.1") != http.StatusNoContent {
		t.Fatal("first request limited")
	}
	if post("8.8.8.8, 9.9.9.9") != http.StatusTooManyRequests {
		t.Fatal("spoofed X-Forwarded-For reset the rate limit")
	}
}

func TestClientIPUsesRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.8:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := clientIP(req); got != "10.0.0.8" {
		t.Fatalf("client IP = %q", got)
	}
}

func TestPathCapEvictsIdleAndRejectsLive(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", MaxPaths: 1})
	ctx := testCtx(t)
	first := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, first, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "aaa", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, first); msg.Type != protocol.TypeRegistered {
		t.Fatalf("first = %+v", msg)
	}
	first.CloseNow()
	waitUntil(t, func() bool { return !tunnelUp(srv, "aaa") })

	second := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, second, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "bbb", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, second); msg.Type != protocol.TypeRegistered {
		t.Fatalf("second = %+v", msg)
	}
	if srv.pathState("aaa") != nil {
		t.Fatal("idle path was not evicted")
	}
	if !tunnelUp(srv, "bbb") {
		t.Fatal("replacement tunnel missing")
	}

	third := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, third, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "ccc", Mode: protocol.ModeTunnel})
	msg := wsRead(t, ctx, third)
	if msg.Type != protocol.TypeError || msg.Error != "too many paths" {
		t.Fatalf("third = %+v", msg)
	}
	if !tunnelUp(srv, "bbb") {
		t.Fatal("live tunnel was evicted")
	}
}

func TestIdlePathExpires(t *testing.T) {
	srv, ts := newRelay(t, Config{Token: "secret", PathTTL: 80 * time.Millisecond})
	ctx := testCtx(t)
	conn := dialTunnel(t, ctx, ts.URL)
	wsWrite(t, ctx, conn, protocol.Message{Type: protocol.TypeRegister, Token: "secret", Path: "dev", Mode: protocol.ModeTunnel})
	if msg := wsRead(t, ctx, conn); msg.Type != protocol.TypeRegistered {
		t.Fatalf("register = %+v", msg)
	}
	conn.CloseNow()
	waitUntil(t, func() bool { return !tunnelUp(srv, "dev") })
	waitUntil(t, func() bool { return srv.pathState("dev") == nil })

	resp, err := http.Post(ts.URL+"/h/dev/hook", "text/plain", strings.NewReader("late"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestHTTPServerTimeouts(t *testing.T) {
	srv := HTTPServer(":8080", http.NewServeMux(), 15*time.Second)
	if srv.ReadHeaderTimeout != 10*time.Second || srv.ReadTimeout != 60*time.Second {
		t.Fatalf("read timeouts = %s %s", srv.ReadHeaderTimeout, srv.ReadTimeout)
	}
	if srv.WriteTimeout <= srv.ReadTimeout || srv.IdleTimeout != 120*time.Second {
		t.Fatalf("write %s idle %s", srv.WriteTimeout, srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 1<<20 {
		t.Fatalf("max header bytes = %d", srv.MaxHeaderBytes)
	}
}

func shortDir(t *testing.T) string {
	t.Helper()
	// Unix socket paths on macOS must fit in 103 bytes. t.TempDir includes
	// the test name and is too long for the longer tests.
	dir, err := os.MkdirTemp("/tmp", "pl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func newRelay(t *testing.T, cfg Config) (*Server, *httptest.Server) {
	t.Helper()
	srv := newRelayOnly(t, cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

func newRelayOnly(t *testing.T, cfg Config) *Server {
	t.Helper()
	srv := New(cfg, nil)
	t.Cleanup(srv.Close)
	return srv
}

func startClient(t *testing.T, relayURL, path, token string, port int, out io.Writer) *tunnel.Client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	client := &tunnel.Client{
		RelayURL:     relayURL,
		Path:         path,
		Token:        token,
		Port:         port,
		Out:          out,
		Err:          io.Discard,
		SocketDir:    shortDir(t),
		PingInterval: time.Hour,
		BackoffMin:   20 * time.Millisecond,
		BackoffMax:   50 * time.Millisecond,
	}
	errLog := &memLog{}
	if client.Err == nil || client.Err == io.Discard {
		client.Err = errLog
	}
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			_, _ = errLog.Write([]byte("run exit: " + err.Error() + "\n"))
		}
	}()
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("client log:\n%s", errLog.String())
		}
	})
	return client
}

func clientSocket(t *testing.T, c *tunnel.Client) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sock, err := tunnel.RunningSocket(c.SocketDir, c.Path)
		if err != nil {
			t.Fatal(err)
		}
		if sock != "" {
			return sock
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket not ready")
	return ""
}

func portOf(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func dialTunnel(t *testing.T, ctx context.Context, base string) *websocket.Conn {
	t.Helper()
	endpoint := strings.Replace(base, "http://", "ws://", 1) + "/tunnel"
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func wsWrite(t *testing.T, ctx context.Context, conn *websocket.Conn, msg protocol.Message) {
	t.Helper()
	b, err := protocol.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func wsRead(t *testing.T, ctx context.Context, conn *websocket.Conn) protocol.Message {
	t.Helper()
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := protocol.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func tunnelUp(s *Server, path string) bool {
	ps := s.pathState(path)
	if ps == nil {
		return false
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.conn != nil
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met")
}

type memLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (m *memLog) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.b.Write(p)
}

func (m *memLog) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.b.String()
}
