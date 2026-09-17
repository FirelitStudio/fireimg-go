package fireimg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// FileSpec is one file in a presign request.
type FileSpec struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
}

// PresignResult is one entry from POST /v1/api/presigned-upload-urls.
type PresignResult struct {
	Filename      string
	UploadURL     string
	S3Key         string
	ExpiresInSecs int
}

// Upload is one file to presign and PUT.
//
// Provide bytes with Body, Bytes, or Path. Sizes is optional: nil keeps the
// project defaults; a non-nil empty slice sends [] and pre-generates only the
// dashboard thumbnail.
type Upload struct {
	Filename    string
	ContentType string
	Body        io.Reader
	Bytes       []byte
	Path        string
	Sizes       []Size
}

// UploadResult is returned after a successful presign + PUT.
type UploadResult struct {
	Filename string
	ImageKey string
	CDNURL   string
	S3Key    string
}

type presignURL struct {
	Filename      string `json:"filename"`
	UploadURL     string `json:"uploadUrl"`
	S3Key         string `json:"s3Key"`
	ExpiresInSecs int    `json:"expiresInSecs"`
}

type presignResponse struct {
	URLs []presignURL `json:"urls"`
}

// Presign requests one presigned S3 PUT URL.
func (c *Client) Presign(ctx context.Context, filename, contentType string, sizes []Size) (*PresignResult, error) {
	rows, err := c.PresignMany(ctx, []FileSpec{{Filename: filename, ContentType: contentType}}, sizes)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, clientError("presign", "returned no URLs")
	}
	return &rows[0], nil
}

// PresignMany requests presigned S3 PUT URLs for up to MaxFilesPerRequest files.
// A nil sizes slice omits the field; a non-nil empty slice sends [].
func (c *Client) PresignMany(ctx context.Context, files []FileSpec, sizes []Size) ([]PresignResult, error) {
	if err := c.requireAPI("presign"); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, clientError("presign", "files cannot be empty")
	}
	if len(files) > MaxFilesPerRequest {
		return nil, clientError("presign", fmt.Sprintf("maximum %d files allowed per request", MaxFilesPerRequest))
	}
	for _, file := range files {
		if strings.TrimSpace(file.Filename) == "" {
			return nil, clientError("presign", "filename is required")
		}
		if strings.TrimSpace(file.ContentType) == "" {
			return nil, clientError("presign", "content type is required")
		}
	}
	if sizes != nil && len(sizes) > MaxSizesPerRequest {
		return nil, clientError("presign", fmt.Sprintf("maximum %d sizes allowed per request", MaxSizesPerRequest))
	}

	payload, err := marshalPresign(c.project(), files, sizes)
	if err != nil {
		return nil, fmt.Errorf("fireimg presign: encode request: %w", err)
	}

	var parsed presignResponse
	if err := c.doJSON(ctx, "presign", http.MethodPost, c.apiBase()+"/v1/api/presigned-upload-urls", json.RawMessage(payload), &parsed); err != nil {
		return nil, err
	}
	if len(parsed.URLs) != len(files) {
		return nil, clientError("presign", fmt.Sprintf("returned %d URLs for %d files", len(parsed.URLs), len(files)))
	}

	out := make([]PresignResult, 0, len(parsed.URLs))
	for _, row := range parsed.URLs {
		out = append(out, PresignResult{
			Filename:      strings.TrimPrefix(strings.TrimSpace(row.Filename), "/"),
			UploadURL:     row.UploadURL,
			S3Key:         row.S3Key,
			ExpiresInSecs: row.ExpiresInSecs,
		})
	}
	return out, nil
}

// Upload requests a presigned URL and PUTs the bytes.
func (c *Client) Upload(ctx context.Context, file Upload) (*UploadResult, error) {
	rows, err := c.UploadMany(ctx, []Upload{file})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, clientError("upload", "returned no results")
	}
	return &rows[0], nil
}

// UploadMany presigns and PUTs up to MaxFilesPerRequest files. Shared Sizes come
// from the first file that sets them (including an explicit empty slice).
func (c *Client) UploadMany(ctx context.Context, files []Upload) ([]UploadResult, error) {
	if err := c.requireAPI("upload"); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, clientError("upload", "files cannot be empty")
	}

	specs := make([]FileSpec, 0, len(files))
	bodies := make([][]byte, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	var sizes []Size
	sizesSet := false
	for _, file := range files {
		spec, data, err := prepareUpload(file)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[spec.Filename]; dup {
			return nil, clientError("upload", fmt.Sprintf("duplicate filename %q", spec.Filename))
		}
		seen[spec.Filename] = struct{}{}
		specs = append(specs, spec)
		bodies = append(bodies, data)
		if !sizesSet && file.Sizes != nil {
			sizes = file.Sizes
			sizesSet = true
		}
	}

	var presignSizes []Size
	if sizesSet {
		presignSizes = sizes
		if presignSizes == nil {
			presignSizes = []Size{}
		}
	}

	rows, err := c.PresignMany(ctx, specs, presignSizes)
	if err != nil {
		return nil, err
	}
	return c.putUploads(ctx, rows, specs, bodies)
}

func (c *Client) putUploads(ctx context.Context, rows []PresignResult, specs []FileSpec, bodies [][]byte) ([]UploadResult, error) {
	byName := make(map[string]int, len(specs))
	for i, spec := range specs {
		byName[spec.Filename] = i
	}
	for _, row := range rows {
		if _, ok := byName[row.Filename]; !ok {
			return nil, clientError("upload", fmt.Sprintf("presign returned unexpected filename %q", row.Filename))
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := make([]UploadResult, len(rows))
	errCh := make(chan error, 1)
	sem := make(chan struct{}, MaxConcurrentPuts)
	var wg sync.WaitGroup
	for i, row := range rows {
		index := byName[row.Filename]
		wg.Add(1)
		go func(i int, row PresignResult, index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if err := c.Put(ctx, row.UploadURL, specs[index].ContentType, bytes.NewReader(bodies[index])); err != nil {
				cancel()
				select {
				case errCh <- err:
				default:
				}
				return
			}
			key := row.Filename
			if key == "" {
				key = specs[index].Filename
			}
			out[i] = UploadResult{
				Filename: key,
				ImageKey: key,
				CDNURL:   c.PublicURL(key),
				S3Key:    row.S3Key,
			}
		}(i, row, index)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
		return out, nil
	}
}

// Put uploads raw bytes to a presigned S3 URL. It does not send the API key.
func (c *Client) Put(ctx context.Context, uploadURL, contentType string, body io.Reader) error {
	if strings.TrimSpace(uploadURL) == "" {
		return clientError("put", "upload URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, body)
	if err != nil {
		return fmt.Errorf("fireimg put: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", c.userAgent())

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("fireimg put: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("fireimg put: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError("put", resp.StatusCode, respBody)
	}
	return nil
}

func prepareUpload(file Upload) (FileSpec, []byte, error) {
	filename := strings.TrimSpace(file.Filename)
	if filename == "" {
		return FileSpec{}, nil, clientError("upload", "filename is required")
	}
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = GuessContentType(filename, "")
	}

	var data []byte
	switch {
	case strings.TrimSpace(file.Path) != "":
		raw, err := os.ReadFile(file.Path)
		if err != nil {
			return FileSpec{}, nil, fmt.Errorf("fireimg upload: read %s: %w", file.Path, err)
		}
		data = raw
	case file.Body != nil:
		raw, err := io.ReadAll(file.Body)
		if err != nil {
			return FileSpec{}, nil, fmt.Errorf("fireimg upload: read body: %w", err)
		}
		data = raw
	case file.Bytes != nil:
		data = file.Bytes
	default:
		return FileSpec{}, nil, clientError("upload", "path, body, or bytes is required")
	}

	return FileSpec{Filename: filename, ContentType: contentType}, data, nil
}

func marshalPresign(slug string, files []FileSpec, sizes []Size) ([]byte, error) {
	payload := map[string]any{
		"slug":  slug,
		"files": files,
	}
	if sizes != nil {
		normalized := make([]Size, 0, len(sizes))
		for _, size := range sizes {
			normalized = append(normalized, normalizeSize(size))
		}
		payload["sizes"] = normalized
	}
	return json.Marshal(payload)
}
