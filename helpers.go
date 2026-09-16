package fireimg

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	unsafeSegment = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	dashRun       = regexp.MustCompile(`-+`)
)

// GuessContentType returns headerType when it is a usable MIME type, otherwise
// an image type inferred from the filename extension.
func GuessContentType(filename, headerType string) string {
	ctype := strings.ToLower(strings.TrimSpace(headerType))
	if index := strings.Index(ctype, ";"); index >= 0 {
		ctype = strings.TrimSpace(ctype[:index])
	}
	if ctype != "" && ctype != "application/octet-stream" {
		return ctype
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".heic", ".heif":
		return "image/heic"
	default:
		return "image/jpeg"
	}
}

// SanitizeFilename cleans an image key. Folders are preserved; each segment
// keeps letters, digits, dot, dash, and underscore. ".." is rejected.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	absolute := strings.HasPrefix(name, "/")
	name = strings.Trim(name, "/")
	if name == "" {
		return "image"
	}
	if absolute {
		return sanitizeSegment(filepath.Base(name))
	}
	segments := strings.Split(name, "/")
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		cleaned := sanitizeSegment(segment)
		if cleaned == "" {
			continue
		}
		out = append(out, cleaned)
	}
	if len(out) == 0 {
		return "image"
	}
	return strings.Join(out, "/")
}

func sanitizeSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" || segment == "." || segment == ".." {
		return ""
	}
	cleaned := dashRun.ReplaceAllString(unsafeSegment.ReplaceAllString(segment, "-"), "-")
	cleaned = strings.ReplaceAll(cleaned, "..", "-")
	cleaned = strings.ReplaceAll(cleaned, "-.", ".")
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" || cleaned == "." || cleaned == "-" || unsafeSegment.ReplaceAllString(cleaned, "") == "" {
		return ""
	}
	return cleaned
}
