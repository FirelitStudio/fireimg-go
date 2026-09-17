package fireimg

import (
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultAPIBase is the production FireImg API.
	DefaultAPIBase = "https://api.fireimg.com"
	// DefaultCDNBase is the production FireImg CDN.
	DefaultCDNBase = "https://img.fireimg.com"
	// MaxFilesPerRequest is the API limit for one presign call.
	MaxFilesPerRequest = 20
	// MaxSizesPerRequest is the API limit for pre-generated variants.
	MaxSizesPerRequest = 20
	// MaxConcurrentPuts is how many S3 PUTs UploadMany runs at once.
	MaxConcurrentPuts = 4

	defaultTimeout = 2 * time.Minute
)

// Config holds client settings. Empty APIKey, Project, APIBase, and CDNBase
// fall back to FIREIMG_API_KEY, FIREIMG_PROJECT, FIREIMG_API_BASE, and
// FIREIMG_CDN_BASE. Staging uses https://api.fireimg.dev and https://img.fireimg.dev.
type Config struct {
	APIKey     string
	Project    string
	APIBase    string
	CDNBase    string
	HTTPClient *http.Client
}

func applyDefaults(cfg Config) Config {
	cfg.APIKey = firstNonEmpty(cfg.APIKey, os.Getenv("FIREIMG_API_KEY"))
	cfg.Project = firstNonEmpty(cfg.Project, os.Getenv("FIREIMG_PROJECT"))
	cfg.APIBase = trimSlash(firstNonEmpty(cfg.APIBase, os.Getenv("FIREIMG_API_BASE"), DefaultAPIBase))
	cfg.CDNBase = trimSlash(firstNonEmpty(cfg.CDNBase, os.Getenv("FIREIMG_CDN_BASE"), DefaultCDNBase))
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func trimSlash(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}
