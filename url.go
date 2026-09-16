package fireimg

import (
	"html"
	"net/url"
	"strconv"
	"strings"
)

const maxDimension = 4000

// PublicURL is the CDN URL with no transform query parameters.
func (c *Client) PublicURL(imageKey string) string {
	return BuildURL(c.cdnBase(), c.project(), imageKey, Options{})
}

// URL is the CDN URL with the given transform options. Only set fields are
// emitted. The query key for format is "format"; Options.Fmt is accepted as an alias.
func (c *Client) URL(imageKey string, opts Options) string {
	return BuildURL(c.cdnBase(), c.project(), imageKey, opts)
}

// RawURL is the unmodified bytes at /raw-images/{project}/{imageKey}.
func (c *Client) RawURL(imageKey string) string {
	return BuildRawURL(c.cdnBase(), c.project(), imageKey)
}

// SrcSet builds a comma-separated srcset from Widths or a min/max/step range.
func (c *Client) SrcSet(imageKey string, opts SrcSetOptions) string {
	return BuildSrcSet(c.cdnBase(), c.project(), imageKey, opts)
}

// BuildURL constructs a CDN URL. cdnBase defaults to DefaultCDNBase.
func BuildURL(cdnBase, project, imageKey string, opts Options) string {
	base := trimSlash(firstNonEmpty(cdnBase, DefaultCDNBase))
	project = strings.Trim(project, "/")
	imageKey = strings.TrimPrefix(strings.TrimSpace(imageKey), "/")
	out := base + "/" + project + "/images/" + imageKey
	if qs := optionsQuery(opts); qs != "" {
		return out + "?" + qs
	}
	return out
}

// BuildRawURL constructs a raw-bytes CDN URL.
func BuildRawURL(cdnBase, project, imageKey string) string {
	base := trimSlash(firstNonEmpty(cdnBase, DefaultCDNBase))
	project = strings.Trim(project, "/")
	imageKey = strings.TrimPrefix(strings.TrimSpace(imageKey), "/")
	return base + "/raw-images/" + project + "/" + imageKey
}

// BuildSrcSet constructs a srcset string for an image key.
func BuildSrcSet(cdnBase, project, imageKey string, opts SrcSetOptions) string {
	widths := srcsetWidths(opts)
	if len(widths) == 0 {
		return ""
	}
	entries := make([]string, 0, len(widths))
	for _, width := range widths {
		item := opts.Options
		item.Width = width
		entries = append(entries, BuildURL(cdnBase, project, imageKey, item)+" "+strconv.Itoa(width)+"w")
	}
	return strings.Join(entries, ", ")
}

// SrcSetFromURL builds a srcset by rewriting widths on an existing FireImg URL.
func SrcSetFromURL(src string, opts SrcSetOptions) string {
	widths := srcsetWidths(opts)
	if len(widths) == 0 {
		return ""
	}
	entries := make([]string, 0, len(widths))
	for _, width := range widths {
		item := opts.Options
		item.Width = width
		entries = append(entries, RewriteURL(src, item)+" "+strconv.Itoa(width)+"w")
	}
	return strings.Join(entries, ", ")
}

// SizesAttr is an HTML sizes attribute for a max display width.
func SizesAttr(maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return "(max-width: " + strconv.Itoa(maxWidth) + "px) 100vw, " + strconv.Itoa(maxWidth) + "px"
}

// IsURL reports whether src is a FireImg CDN URL.
func IsURL(src string) bool {
	parsed, err := url.Parse(html.UnescapeString(strings.TrimSpace(src)))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "img.fireimg.com" || host == "i.fireimg.com" || host == "images.fireimg.com" || strings.HasSuffix(host, ".fireimg.com")
}

// RewriteURL applies opts to an existing URL. Existing query params are kept
// unless overwritten. Width and height are updated only when greater than zero.
func RewriteURL(src string, opts Options) string {
	parsed, err := url.Parse(html.UnescapeString(strings.TrimSpace(src)))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return src
	}
	query := parsed.Query()
	applyOptions(query, opts)
	parsed.RawQuery = encodeQuery(query)
	return parsed.String()
}

func srcsetWidths(opts SrcSetOptions) []int {
	if len(opts.Widths) > 0 {
		out := make([]int, 0, len(opts.Widths))
		for _, width := range opts.Widths {
			if width > 0 {
				out = append(out, clampDim(width))
			}
		}
		return out
	}
	minWidth := opts.MinWidth
	maxWidth := opts.MaxWidth
	step := opts.SnapStep
	if step <= 0 {
		step = 100
	}
	if minWidth <= 0 {
		minWidth = 100
	}
	if maxWidth <= 0 {
		maxWidth = 2000
	}
	if maxWidth < minWidth {
		return nil
	}
	var widths []int
	for width := minWidth; width <= maxWidth; width += step {
		widths = append(widths, clampDim(width))
		if width >= maxDimension {
			break
		}
	}
	return widths
}

func optionsQuery(opts Options) string {
	query := url.Values{}
	applyOptions(query, opts)
	return encodeQuery(query)
}

func applyOptions(query url.Values, opts Options) {
	if opts.Width > 0 {
		query.Del("w")
		query.Set("width", strconv.Itoa(clampDim(opts.Width)))
	}
	if opts.Height > 0 {
		query.Del("h")
		query.Set("height", strconv.Itoa(clampDim(opts.Height)))
	}
	if quality := strings.TrimSpace(opts.Quality); quality != "" {
		query.Del("q")
		query.Set("quality", quality)
	}
	if format := opts.format(); format != "" {
		query.Del("fmt")
		query.Set("format", format)
	}
	if fit := strings.TrimSpace(opts.Fit); fit != "" {
		query.Set("fit", fit)
	}
	if pos := opts.position(); pos != "" {
		query.Del("pos")
		query.Set("position", pos)
	}
	if fill := strings.TrimSpace(opts.Fill); fill != "" {
		query.Set("fill", fill)
	}
	if opts.Version >= 1 {
		query.Set("version", strconv.Itoa(opts.Version))
	}
}

func encodeQuery(query url.Values) string {
	keys := []string{"width", "height", "quality", "format", "fit", "position", "fill", "version"}
	var parts []string
	for _, key := range keys {
		if !query.Has(key) {
			continue
		}
		for _, value := range query[key] {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
		query.Del(key)
	}
	if encoded := query.Encode(); encoded != "" {
		parts = append(parts, encoded)
	}
	return strings.Join(parts, "&")
}

func clampDim(value int) int {
	if value < 1 {
		return 1
	}
	if value > maxDimension {
		return maxDimension
	}
	return value
}
