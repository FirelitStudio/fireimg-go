package fireimg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client talks to the FireImg API and builds CDN URLs.
type Client struct {
	config Config
}

// New returns a client. Empty config fields fall back to FIREIMG_* environment
// variables. URL helpers work without an API key; upload and management calls
// require both APIKey and Project.
func New(cfg Config) (*Client, error) {
	return &Client{config: applyDefaults(cfg)}, nil
}

// Configured reports whether API-key operations can run.
func (c *Client) Configured() bool {
	return c != nil && strings.TrimSpace(c.config.APIKey) != "" && strings.TrimSpace(c.config.Project) != ""
}

func (c *Client) apiBase() string {
	return trimSlash(c.config.APIBase)
}

func (c *Client) cdnBase() string {
	return trimSlash(c.config.CDNBase)
}

func (c *Client) project() string {
	return strings.TrimSpace(c.config.Project)
}

func (c *Client) requireAPI(op string) error {
	if c == nil {
		return clientError(op, "client is nil")
	}
	if strings.TrimSpace(c.config.APIKey) == "" || c.project() == "" {
		return clientError(op, "not configured (set api_key and project)")
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.config.HTTPClient != nil {
		return c.config.HTTPClient
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) userAgent() string {
	return "fireimg-go/" + Version
}

func (c *Client) setAPIHeaders(req *http.Request) {
	req.Header.Set("X-Api-Key", c.config.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent())
}

func (c *Client) doJSON(ctx context.Context, op, method, endpoint string, payload any, dest any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("fireimg %s: encode request: %w", op, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("fireimg %s: %w", op, err)
	}
	c.setAPIHeaders(req)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("fireimg %s: %w", op, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("fireimg %s: read response: %w", op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(op, resp.StatusCode, respBody)
	}
	if dest == nil || len(bytes.TrimSpace(respBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, dest); err != nil {
		return fmt.Errorf("fireimg %s: decode response: %w", op, err)
	}
	return nil
}

func imageAPIURL(apiBase, slug, suffix, imageKey string) (string, error) {
	base, err := url.Parse(trimSlash(apiBase))
	if err != nil {
		return "", err
	}
	parts := []string{"v1", "api", "projects", url.PathEscape(slug), "images"}
	if suffix != "" {
		parts = append(parts, suffix)
	}
	for _, segment := range strings.Split(strings.Trim(imageKey, "/"), "/") {
		if segment == "" {
			continue
		}
		parts = append(parts, url.PathEscape(segment))
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.Join(parts, "/")
	return base.String(), nil
}
