// Package config loads Neromancer runtime configuration from .env, environment
// variables, and SQLite persisted settings, and manages local media/export directories.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultDeepSeekBaseURL = "https://api.moark.com"
	DefaultDeepSeekModel   = "deepseek-v4.1-flash"
	DefaultDataDir         = ""
)

// Config holds active runtime configuration for storage, media, and DeepSeek AI.
type Config struct {
	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	DeepSeekModel   string
	DataDir         string
}

// Load reads .env (if present in cwd) and process environment variables with sane defaults.
// When DataDir is empty, main.go resolves it via mygo.App.Path(mygo.PathUserData) for Android/iOS sandbox safety.
func Load(envPath string) Config {
	envFile := map[string]string{}
	if envPath != "" {
		envFile = parseDotEnv(envPath)
	}

	pick := func(key, fallback string) string {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		if v := strings.TrimSpace(envFile[key]); v != "" {
			return v
		}
		return fallback
	}

	return Config{
		DeepSeekAPIKey:  pick("DEEPSEEK_API_KEY", ""),
		DeepSeekBaseURL: strings.TrimRight(pick("DEEPSEEK_BASE_URL", DefaultDeepSeekBaseURL), "/"),
		DeepSeekModel:   pick("DEEPSEEK_MODEL", DefaultDeepSeekModel),
		DataDir:         pick("NEROMANCER_DATA_DIR", DefaultDataDir),
	}
}

// DBPath returns the local vault file location inside DataDir.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "vault.json")
}

// AudioDir returns the managed raw voice evidence directory.
func (c Config) AudioDir() string {
	return filepath.Join(c.DataDir, "media", "audio")
}

// ImageDir returns the managed raw image/sketch evidence directory.
func (c Config) ImageDir() string {
	return filepath.Join(c.DataDir, "media", "images")
}

// ExportDir returns the directory for exported PNG cards and .orb.json capsules.
func (c Config) ExportDir() string {
	return filepath.Join(c.DataDir, "exports")
}

// EnsureDirs creates all required local data directories if they do not exist.
func (c Config) EnsureDirs() error {
	for _, dir := range []string{c.DataDir, c.AudioDir(), c.ImageDir(), c.ExportDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func parseDotEnv(path string) map[string]string {
	out := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		out[key] = val
	}
	return out
}
