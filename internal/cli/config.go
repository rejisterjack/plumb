package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type fileConfig struct {
	Relay string
	Path  string
	Port  int
}

type connectOpts struct {
	Relay   string
	Path    string
	Port    int
	Token   string
	MaxBody int
}

type replayOpts struct {
	connectOpts
	Last int
}

type relayOpts struct {
	Addr           string
	Token          string
	Dev            bool
	Buffer         int
	MaxBody        int
	ForwardTimeout time.Duration
	MaxPaths       int
	PathTTL        time.Duration
}

// version is overwritten by release builds:
// -X github.com/rejisterjack/plumb/internal/cli.version=v0.1.0
var version = "dev"

func loadDefaultConfig() (fileConfig, error) {
	return loadFileConfig(".plumb.toml")
}

func loadFileConfig(path string) (fileConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileConfig{}, nil
	}
	if err != nil {
		return fileConfig{}, err
	}
	defer f.Close()

	var cfg fileConfig
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return fileConfig{}, fmt.Errorf("%s:%d: expected key = value", path, lineNo)
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "relay":
			cfg.Relay = val
		case "path":
			cfg.Path = val
		case "port":
			n, err := strconv.Atoi(val)
			if err != nil {
				return fileConfig{}, fmt.Errorf("%s:%d: port: %w", path, lineNo, err)
			}
			cfg.Port = n
		default:
			return fileConfig{}, fmt.Errorf("%s:%d: unknown key %q", path, lineNo, key)
		}
	}
	if err := sc.Err(); err != nil {
		return fileConfig{}, err
	}
	return cfg, nil
}

func resolveListenAddr(flagAddr string) string {
	if flagAddr != "" {
		return flagAddr
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return ":8080"
	}
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

func envInt(key string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}
