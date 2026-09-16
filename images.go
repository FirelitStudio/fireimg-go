package fireimg

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ModerationLabel is one Rekognition label from a content scan.
type ModerationLabel struct {
	Name       string  `json:"name"`
	Parent     string  `json:"parent"`
	Confidence float64 `json:"confidence"`
}

// ModerationResult is the body of POST .../images/moderation/{imageKey}.
type ModerationResult struct {
	Moderation string            `json:"moderation"`
	Labels     []ModerationLabel `json:"labels,omitempty"`
	Reason     string            `json:"reason,omitempty"`
}

// Project is one entry from GET /v1/api/projects.
type Project struct {
	ProjectName string `json:"projectName"`
	Slug        string `json:"slug"`
	TeamID      string `json:"teamId,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// ListImagesParams are query options for GET /v1/api/projects/{slug}/images.
type ListImagesParams struct {
	Limit     int
	NextToken string
	Folder    string
}

// Image is one entry from GET /v1/api/projects/{slug}/images.
type Image struct {
	ImageKey                 string `json:"imageKey"`
	S3RawKey                 string `json:"s3RawKey"`
	UploadedAt               string `json:"uploadedAt"`
	OriginalSizeBytes        int64  `json:"originalSizeBytes"`
	ContentType              string `json:"contentType"`
	VariationsGeneratedCount int    `json:"variationsGeneratedCount,omitempty"`
	CacheVersion             int    `json:"cacheVersion,omitempty"`
}

// ListImagesResponse is one page of images.
type ListImagesResponse struct {
	Images    []Image `json:"images"`
	Folder    string  `json:"folder"`
	NextToken string  `json:"nextToken,omitempty"`
}

// Moderate runs a Rekognition content scan. JPEG and PNG only. The result is
// labels for the app to apply policy; FireImg does not reject the image.
func (c *Client) Moderate(ctx context.Context, imageKey string) (*ModerationResult, error) {
	if err := c.requireAPI("moderate"); err != nil {
		return nil, err
	}
	imageKey = strings.TrimPrefix(strings.TrimSpace(imageKey), "/")
	if imageKey == "" {
		return nil, clientError("moderate", "image key is required")
	}
	endpoint, err := imageAPIURL(c.apiBase(), c.project(), "moderation", imageKey)
	if err != nil {
		return nil, err
	}
	var result ModerationResult
	if err := c.doJSON(ctx, "moderate", http.MethodPost, endpoint, map[string]any{}, &result); err != nil {
		return nil, err
	}
	if result.Labels == nil {
		result.Labels = []ModerationLabel{}
	}
	return &result, nil
}

// Delete soft-deletes an image by key.
func (c *Client) Delete(ctx context.Context, imageKey string) error {
	if err := c.requireAPI("delete"); err != nil {
		return err
	}
	imageKey = strings.TrimPrefix(strings.TrimSpace(imageKey), "/")
	if imageKey == "" {
		return clientError("delete", "image key is required")
	}
	endpoint, err := imageAPIURL(c.apiBase(), c.project(), "", imageKey)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, "delete", http.MethodDelete, endpoint, nil, nil)
}

// ListProjects returns every project the API key's team can access.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	if err := c.requireAPI("list projects"); err != nil {
		return nil, err
	}
	var parsed struct {
		Projects []Project `json:"projects"`
	}
	if err := c.doJSON(ctx, "list projects", http.MethodGet, c.apiBase()+"/v1/api/projects", nil, &parsed); err != nil {
		return nil, err
	}
	if parsed.Projects == nil {
		return []Project{}, nil
	}
	return parsed.Projects, nil
}

// ListImages returns one page of images for the configured project.
func (c *Client) ListImages(ctx context.Context, params ListImagesParams) (*ListImagesResponse, error) {
	if err := c.requireAPI("list images"); err != nil {
		return nil, err
	}
	endpoint, err := listImagesURL(c.apiBase(), c.project(), params)
	if err != nil {
		return nil, err
	}
	var parsed ListImagesResponse
	if err := c.doJSON(ctx, "list images", http.MethodGet, endpoint, nil, &parsed); err != nil {
		return nil, err
	}
	if parsed.Images == nil {
		parsed.Images = []Image{}
	}
	return &parsed, nil
}

func listImagesURL(apiBase, slug string, params ListImagesParams) (string, error) {
	base, err := url.Parse(trimSlash(apiBase))
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + "/v1/api/projects/" + url.PathEscape(slug) + "/images"
	query := url.Values{}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.NextToken != "" {
		query.Set("nextToken", params.NextToken)
	}
	if params.Folder != "" {
		query.Set("folder", params.Folder)
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
}
