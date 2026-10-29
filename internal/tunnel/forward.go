package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rejisterjack/plumb/internal/httpheader"
)

var localClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		DisableCompression:    true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
	},
}

// SafePath rejects anything that is not a path on the local server.
func SafePath(raw string) (string, error) {
	if strings.ContainsRune(raw, 0) || strings.Contains(raw, "\\") || strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("refusing non-local path")
	}
	if raw == "" {
		raw = "/"
	}
	pathPart, query, hasQuery := strings.Cut(raw, "?")
	if pathPart == "" {
		pathPart = "/"
	}
	if !strings.HasPrefix(pathPart, "/") {
		pathPart = "/" + pathPart
	}
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		return "", fmt.Errorf("refusing non-local path")
	}
	if strings.Contains(decoded, "..") {
		return "", fmt.Errorf("refusing non-local path")
	}
	cleaned := path.Clean(pathPart)
	if !strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("refusing non-local path")
	}
	if hasQuery {
		return cleaned + "?" + query, nil
	}
	return cleaned, nil
}

// Forward sends one webhook to 127.0.0.1. The body is forwarded unchanged.
func Forward(ctx context.Context, port int, maxBody int64, method, urlPath, host, original string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	if port < 1 || port > 65535 {
		return 0, nil, nil, fmt.Errorf("invalid local port %d", port)
	}
	if !validMethod(method) {
		return 0, nil, nil, fmt.Errorf("refusing method %q", method)
	}
	forwardPath, err := SafePath(urlPath)
	if err != nil {
		return 0, nil, nil, err
	}
	if maxBody <= 0 {
		maxBody = 2 << 20
	}
	endpoint := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + forwardPath
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Host = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	req.Header = httpheader.CloneForward(headers)
	if host != "" {
		req.Header.Set("X-Forwarded-Host", host)
	}
	if original != "" {
		req.Header.Set("X-Plumb-Original-Url", original)
		if u, err := url.Parse(original); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			req.Header.Set("X-Forwarded-Proto", u.Scheme)
		}
	}
	req.ContentLength = int64(len(body))

	resp, err := localClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return 0, nil, nil, err
	}
	if int64(len(respBody)) > maxBody {
		return 0, nil, nil, fmt.Errorf("local response exceeds %d bytes", maxBody)
	}
	return resp.StatusCode, httpheader.CloneForward(resp.Header), respBody, nil
}

func validMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}
