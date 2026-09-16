package fireimg

import "strings"

// Size is one variant to pre-generate after upload. These replace the project's
// default image params for that upload. An empty slice still overrides defaults
// and pre-generates only FireImg's 80px dashboard thumbnail.
type Size struct {
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Quality string `json:"quality,omitempty"`
	Fmt     string `json:"fmt,omitempty"`
	Fit     string `json:"fit,omitempty"`
	Pos     string `json:"pos,omitempty"`
	Fill    string `json:"fill,omitempty"`
}

// Options are CDN transform query parameters. Only set fields are emitted;
// quality and format are not defaulted.
type Options struct {
	Width    int
	Height   int
	Quality  string
	Format   string
	Fmt      string
	Fit      string
	Pos      string
	Position string
	Fill     string
	Version  int
}

func (o Options) format() string {
	return firstNonEmpty(o.Format, o.Fmt)
}

func (o Options) position() string {
	return firstNonEmpty(o.Pos, o.Position)
}

// SrcSetOptions build a responsive srcset. Use Widths for an explicit ladder,
// or MinWidth/MaxWidth/SnapStep (defaults 100/2000/100) like @fireimg/js.
type SrcSetOptions struct {
	Widths   []int
	MinWidth int
	MaxWidth int
	SnapStep int
	Options  Options
}

// SizesForWidths builds width-only pre-generation sizes.
func SizesForWidths(widths []int) []Size {
	sizes := make([]Size, 0, len(widths))
	for _, width := range widths {
		if width <= 0 {
			continue
		}
		sizes = append(sizes, Size{Width: width})
	}
	return sizes
}

func normalizeSize(size Size) Size {
	if size.Width < 0 {
		size.Width = 0
	}
	if size.Height < 0 {
		size.Height = 0
	}
	size.Quality = strings.TrimSpace(size.Quality)
	size.Fmt = firstNonEmpty(size.Fmt)
	size.Fit = strings.TrimSpace(size.Fit)
	size.Pos = strings.TrimSpace(size.Pos)
	size.Fill = strings.TrimSpace(size.Fill)
	return size
}
