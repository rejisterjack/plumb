package tunnel

import (
	"fmt"
	"net/url"
	"strings"
)

func parseRelay(relay string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(relay))
	if err != nil {
		return nil, fmt.Errorf("relay URL: %w", err)
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("relay must be an http or https URL")
	}
	return u, nil
}

// PublicURL is the stable webhook prefix for a path, with a trailing slash.
func PublicURL(relay, path string) (string, error) {
	u, err := parseRelay(relay)
	if err != nil {
		return "", err
	}
	u.Path = "/h/" + path + "/"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// TunnelURL is the WebSocket endpoint for a relay origin.
func TunnelURL(relay string) (string, error) {
	u, err := parseRelay(relay)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/tunnel"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
