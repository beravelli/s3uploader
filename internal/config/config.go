package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AWSRegion      string
	S3Bucket       string
	Port           string
	MaxUploadBytes int64
}

// Load reads configuration from environment variables, falling back to a
// .env file in the working directory for any variable not already set.
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		AWSRegion: os.Getenv("AWS_REGION"),
		S3Bucket:  os.Getenv("S3_BUCKET"),
		Port:      os.Getenv("PORT"),
	}
	if cfg.AWSRegion == "" {
		return nil, fmt.Errorf("AWS_REGION is required")
	}
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	maxMB := int64(100)
	if v := os.Getenv("MAX_UPLOAD_MB"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid MAX_UPLOAD_MB %q: %w", v, err)
		}
		maxMB = n
	}
	cfg.MaxUploadBytes = maxMB << 20

	return cfg, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
