package fireimg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(Config{
		APIKey:     "fimg_test",
		Project:    "needapp",
		APIBase:    server.URL,
		CDNBase:    "https://img.fireimg.com",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestNewReadsEnv(t *testing.T) {
	t.Setenv("FIREIMG_API_KEY", "fimg_env")
	t.Setenv("FIREIMG_PROJECT", "from-env")
	t.Setenv("FIREIMG_API_BASE", "https://api.fireimg.dev/")
	t.Setenv("FIREIMG_CDN_BASE", "https://img.fireimg.dev/")
	client, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !client.Configured() {
		t.Fatal("expected configured client")
	}
	if client.config.APIKey != "fimg_env" || client.project() != "from-env" {
		t.Fatalf("env not applied: %+v", client.config)
	}
	if client.apiBase() != "https://api.fireimg.dev" || client.cdnBase() != "https://img.fireimg.dev" {
		t.Fatalf("bases: api=%s cdn=%s", client.apiBase(), client.cdnBase())
	}
}

func TestPresignSendsSizesAndOmitsWhenNil(t *testing.T) {
	var bodies []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/presigned-upload-urls" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "fimg_test" {
			t.Fatalf("api key %q", r.Header.Get("X-Api-Key"))
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "fireimg-go/") {
			t.Fatalf("user agent %q", r.Header.Get("User-Agent"))
		}
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		_ = json.NewEncoder(w).Encode(presignResponse{URLs: []presignURL{{
			Filename: "hero.jpg", UploadURL: "https://s3.example/put", S3Key: "raw-images/needapp/hero.jpg", ExpiresInSecs: 900,
		}}})
	})

	if _, err := client.Presign(context.Background(), "hero.jpg", "image/jpeg", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Presign(context.Background(), "hero.jpg", "image/jpeg", []Size{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Presign(context.Background(), "hero.jpg", "image/jpeg", []Size{{Width: 800, Quality: "medium", Fmt: "webp"}}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(bodies[0], `"sizes"`) {
		t.Fatalf("nil sizes should omit field: %s", bodies[0])
	}
	if !strings.Contains(bodies[1], `"sizes":[]`) {
		t.Fatalf("empty sizes should send []: %s", bodies[1])
	}
	if !strings.Contains(bodies[2], `"width":800`) || !strings.Contains(bodies[2], `"fmt":"webp"`) {
		t.Fatalf("size payload: %s", bodies[2])
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first["slug"] != "needapp" {
		t.Fatalf("slug %v", first["slug"])
	}
}

func TestUploadPresignThenPutWithoutAPIKey(t *testing.T) {
	var putAPIKey string
	var putType string
	var putBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api/presigned-upload-urls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(presignResponse{URLs: []presignURL{{
			Filename: "feed_items/43/a.jpg", UploadURL: "http://" + r.Host + "/s3", S3Key: "raw-images/needapp/feed_items/43/a.jpg",
		}}})
	})
	mux.HandleFunc("/s3", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method %s", r.Method)
		}
		putAPIKey = r.Header.Get("X-Api-Key")
		putType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		putBody = string(raw)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := New(Config{APIKey: "fimg_test", Project: "needapp", APIBase: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Upload(context.Background(), Upload{
		Filename:    "feed_items/43/a.jpg",
		ContentType: "image/jpeg",
		Bytes:       []byte("jpeg-bytes"),
		Sizes:       []Size{{Width: 800}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if putAPIKey != "" {
		t.Fatalf("PUT must not send API key, got %q", putAPIKey)
	}
	if putType != "image/jpeg" || putBody != "jpeg-bytes" {
		t.Fatalf("put type=%s body=%s", putType, putBody)
	}
	if result.ImageKey != "feed_items/43/a.jpg" {
		t.Fatalf("key %s", result.ImageKey)
	}
	if result.CDNURL != "https://img.fireimg.com/needapp/images/feed_items/43/a.jpg" {
		t.Fatalf("cdn %s", result.CDNURL)
	}
}

func TestUploadFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.jpg")
	if err := os.WriteFile(path, []byte("from-disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api/presigned-upload-urls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(presignResponse{URLs: []presignURL{{
			Filename: "hero.jpg", UploadURL: "http://" + r.Host + "/s3",
		}}})
	})
	mux.HandleFunc("/s3", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = string(raw)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := New(Config{APIKey: "k", Project: "p", APIBase: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Upload(context.Background(), Upload{
		Filename: "hero.jpg", ContentType: "image/jpeg", Path: path,
	}); err != nil {
		t.Fatal(err)
	}
	if got != "from-disk" {
		t.Fatalf("body %q", got)
	}
}

func TestUploadManyPutsInParallel(t *testing.T) {
	const n = 5
	var current, peak, puts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api/presigned-upload-urls", func(w http.ResponseWriter, r *http.Request) {
		urls := make([]presignURL, n)
		for i := range urls {
			name := "f" + string(rune('a'+i)) + ".jpg"
			urls[i] = presignURL{Filename: name, UploadURL: "http://" + r.Host + "/s3/" + name}
		}
		_ = json.NewEncoder(w).Encode(presignResponse{URLs: urls})
	})
	mux.HandleFunc("/s3/", func(w http.ResponseWriter, r *http.Request) {
		live := current.Add(1)
		for {
			old := peak.Load()
			if live <= old || peak.CompareAndSwap(old, live) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		current.Add(-1)
		puts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := New(Config{APIKey: "k", Project: "p", APIBase: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	files := make([]Upload, n)
	for i := range files {
		files[i] = Upload{Filename: "f" + string(rune('a'+i)) + ".jpg", ContentType: "image/jpeg", Bytes: []byte("x")}
	}
	results, err := client.UploadMany(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != n || puts.Load() != n {
		t.Fatalf("results=%d puts=%d", len(results), puts.Load())
	}
	if peak.Load() > MaxConcurrentPuts {
		t.Fatalf("peak concurrency %d > %d", peak.Load(), MaxConcurrentPuts)
	}
	if peak.Load() < 2 {
		t.Fatalf("expected overlapping PUTs, peak %d", peak.Load())
	}
	if results[0].ImageKey != "fa.jpg" || results[4].ImageKey != "fe.jpg" {
		t.Fatalf("order %+v", results)
	}
}

func TestUploadManyAndErrors(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API")
	})
	_, err := client.UploadMany(context.Background(), []Upload{
		{Filename: "a.jpg", ContentType: "image/jpeg", Bytes: []byte("a")},
		{Filename: "a.jpg", ContentType: "image/jpeg", Bytes: []byte("b")},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate: %v", err)
	}

	files := make([]FileSpec, MaxFilesPerRequest+1)
	for i := range files {
		files[i] = FileSpec{Filename: "f.jpg", ContentType: "image/jpeg"}
	}
	if _, err := client.PresignMany(context.Background(), files, nil); err == nil {
		t.Fatal("expected max files error")
	}
	if _, err := client.PresignMany(context.Background(), []FileSpec{{Filename: "a.jpg", ContentType: "image/jpeg"}}, make([]Size, MaxSizesPerRequest+1)); err == nil {
		t.Fatal("expected max sizes error")
	}
	if _, err := client.Upload(context.Background(), Upload{Filename: "a.jpg", ContentType: "image/jpeg"}); err == nil {
		t.Fatal("expected missing bytes")
	}

	unconfigured, err := New(Config{Project: "needapp"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unconfigured.Upload(context.Background(), Upload{Filename: "a.jpg", Bytes: []byte("x")}); err == nil {
		t.Fatal("expected not configured")
	}
}

func TestUploadFromBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api/presigned-upload-urls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(presignResponse{URLs: []presignURL{{
			Filename: "hero.jpg", UploadURL: "http://" + r.Host + "/s3",
		}}})
	})
	mux.HandleFunc("/s3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := New(Config{APIKey: "k", Project: "p", APIBase: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Upload(context.Background(), Upload{
		Filename: "hero.jpg", Body: bytes.NewReader([]byte("io")),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAPIUnauthorized(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"x-api-key header is required"}`))
	})
	_, err := client.Presign(context.Background(), "a.jpg", "image/jpeg", nil)
	var apiErr *Error
	if err == nil {
		t.Fatal("expected error")
	}
	if !asError(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("error %#v", err)
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if !ok {
		return false
	}
	*target = e
	return true
}

func TestModerateDeleteAndList(t *testing.T) {
	var paths []string
	var methods []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		methods = append(methods, r.Method)
		switch {
		case strings.Contains(r.URL.Path, "/moderation/"):
			_ = json.NewEncoder(w).Encode(ModerationResult{
				Moderation: "checked",
				Labels:     []ModerationLabel{{Name: "Graphic Male Nudity", Parent: "Explicit Nudity", Confidence: 91.2}},
			})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/v1/api/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"projects": []Project{{ProjectName: "NeedApp", Slug: "needapp", CreatedAt: "2025-06-15T10:30:00Z"}},
			})
		case strings.HasSuffix(r.URL.Path, "/images"):
			if r.URL.Query().Get("folder") != "feed_items" || r.URL.Query().Get("limit") != "50" {
				t.Fatalf("query %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(ListImagesResponse{
				Images: []Image{{ImageKey: "feed_items/43/uuid.jpg", ContentType: "image/jpeg"}},
				Folder: "feed_items",
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	mod, err := client.Moderate(context.Background(), "feed_items/43/uuid.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if mod.Moderation != "checked" || len(mod.Labels) != 1 {
		t.Fatalf("%+v", mod)
	}
	if err := client.Delete(context.Background(), "feed_items/43/uuid.jpg"); err != nil {
		t.Fatal(err)
	}
	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Slug != "needapp" {
		t.Fatalf("%+v", projects)
	}
	images, err := client.ListImages(context.Background(), ListImagesParams{Limit: 50, Folder: "feed_items"})
	if err != nil {
		t.Fatal(err)
	}
	if len(images.Images) != 1 {
		t.Fatalf("%+v", images)
	}

	if !strings.HasSuffix(paths[0], "/images/moderation/feed_items/43/uuid.jpg") {
		t.Fatalf("moderate path %s", paths[0])
	}
	if methods[1] != http.MethodDelete {
		t.Fatalf("delete method %s", methods[1])
	}
}

func TestPutFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	t.Cleanup(server.Close)
	client, err := New(Config{APIKey: "k", Project: "p", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Put(context.Background(), server.URL, "image/jpeg", bytes.NewReader([]byte("x")))
	var apiErr *Error
	if err == nil || !asError(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("error %#v", err)
	}
}
