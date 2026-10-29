package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rejisterjack/plumb/internal/protocol"
)

type controlRequest struct {
	Last int `json:"last"`
}

type controlResponse struct {
	Delivered int    `json:"delivered"`
	Error     string `json:"error,omitempty"`
}

func (c *Client) serveControl(ctx context.Context) error {
	sock, err := c.socketPath()
	if err != nil {
		return err
	}
	ln, err := listenSocket(sock)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.ln = ln
	c.sockPath = sock
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go c.handleControl(conn)
		}
	}()
	return nil
}

func (c *Client) closeControl() {
	c.mu.Lock()
	ln := c.ln
	sock := c.sockPath
	c.ln = nil
	c.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	if sock != "" {
		_ = os.Remove(sock)
	}
}

func (c *Client) socketPath() (string, error) {
	dir := c.SocketDir
	if dir == "" {
		var err error
		dir, err = defaultSocketDir(c.Path)
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(dir, 0o700)
	sock := filepath.Join(dir, c.Path+".sock")
	if len(sock) > maxUnixSocketPath() {
		return "", fmt.Errorf("control socket path is too long for this OS (%d bytes)", len(sock))
	}
	return sock, nil
}

// defaultSocketDir is ~/.plumb, or a short per-user directory when that path
// cannot fit in a Unix socket address.
func defaultSocketDir(slug string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".plumb")
	if slug != "" && len(filepath.Join(dir, slug+".sock")) > maxUnixSocketPath() {
		base := "/tmp"
		if runtime.GOOS == "windows" {
			base = os.TempDir()
		}
		dir = filepath.Join(base, fmt.Sprintf("plumb-%d", os.Getuid()))
	}
	return dir, nil
}

func maxUnixSocketPath() int {
	// sockaddr_un.sun_path includes a trailing NUL.
	switch runtime.GOOS {
	case "darwin", "ios", "freebsd", "openbsd", "netbsd", "dragonfly":
		return 103
	case "windows":
		return 200
	default:
		return 107
	}
}

func (c *Client) handleControl(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	var req controlRequest
	if err := json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(controlResponse{Error: "bad request"})
		return
	}
	if req.Last <= 0 {
		req.Last = 5
	}
	if req.Last > 1000 {
		req.Last = 1000
	}
	budget := time.Duration(req.Last)*15*time.Second + 10*time.Second
	_ = conn.SetDeadline(time.Now().Add(budget))
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	n, err := c.DeliverLast(ctx, req.Last)
	resp := controlResponse{Delivered: n}
	if err != nil {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func listenSocket(path string) (net.Listener, error) {
	if _, err := os.Stat(path); err == nil {
		if socketAlive(path) {
			return nil, fmt.Errorf("plumb connect is already running for this path")
		}
		_ = os.Remove(path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil && runtime.GOOS != "windows" {
		ln.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return ln, nil
}

func socketAlive(path string) bool {
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RunningSocket returns the control socket for a live `plumb connect`.
// An empty path selects the only live socket. The result is empty when
// nothing is running.
func RunningSocket(dir, path string) (string, error) {
	if dir == "" {
		var err error
		dir, err = defaultSocketDir(path)
		if err != nil {
			return "", err
		}
	}
	if path != "" {
		if !protocol.ValidSlug(path) {
			return "", fmt.Errorf("invalid path %q", path)
		}
		sock := filepath.Join(dir, path+".sock")
		if socketAlive(sock) {
			return sock, nil
		}
		return "", nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.sock"))
	if err != nil {
		return "", err
	}
	var alive []string
	for _, match := range matches {
		if socketAlive(match) {
			alive = append(alive, match)
		}
	}
	switch len(alive) {
	case 0:
		return "", nil
	case 1:
		return alive[0], nil
	default:
		return "", errors.New("multiple plumb connect processes; pass --path")
	}
}

// ReplayLocal asks a running connect process to re-POST the last n webhooks.
func ReplayLocal(sock string, last int) (int, error) {
	conn, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	budget := time.Duration(last)*15*time.Second + 15*time.Second
	if budget < 20*time.Second {
		budget = 20 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(budget))
	if err := json.NewEncoder(conn).Encode(controlRequest{Last: last}); err != nil {
		return 0, err
	}
	var resp controlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return resp.Delivered, errors.New(resp.Error)
	}
	return resp.Delivered, nil
}

func randomSlug() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
