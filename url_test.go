package fireimg

import "testing"

func TestBuildURL(t *testing.T) {
	client, err := New(Config{Project: "needapp", CDNBase: "https://img.fireimg.com"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		key  string
		opts Options
		want string
	}{
		{
			name: "public",
			key:  "feed_items/43/uuid.jpg",
			want: "https://img.fireimg.com/needapp/images/feed_items/43/uuid.jpg",
		},
		{
			name: "width quality format",
			key:  "hero.jpg",
			opts: Options{Width: 800, Quality: "medium", Format: "webp"},
			want: "https://img.fireimg.com/needapp/images/hero.jpg?width=800&quality=medium&format=webp",
		},
		{
			name: "fmt alias",
			key:  "hero.jpg",
			opts: Options{Width: 800, Fmt: "auto"},
			want: "https://img.fireimg.com/needapp/images/hero.jpg?width=800&format=auto",
		},
		{
			name: "cover fit",
			key:  "banner.jpg",
			opts: Options{Width: 300, Height: 200, Quality: "medium", Format: "jpg", Fit: "cover", Position: "center"},
			want: "https://img.fireimg.com/needapp/images/banner.jpg?width=300&height=200&quality=medium&format=jpg&fit=cover&position=center",
		},
		{
			name: "version",
			key:  "hero.jpg",
			opts: Options{Width: 400, Version: 2},
			want: "https://img.fireimg.com/needapp/images/hero.jpg?width=400&version=2",
		},
		{
			name: "leading slash",
			key:  "/hero.jpg",
			opts: Options{Width: 764, Quality: "high", Format: "auto"},
			want: "https://img.fireimg.com/needapp/images/hero.jpg?width=764&quality=high&format=auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "public" {
				if got := client.PublicURL(tt.key); got != tt.want {
					t.Fatalf("PublicURL=%s want %s", got, tt.want)
				}
			}
			if got := client.URL(tt.key, tt.opts); got != tt.want {
				t.Fatalf("URL=%s want %s", got, tt.want)
			}
		})
	}
}

func TestRawURL(t *testing.T) {
	client, err := New(Config{Project: "needapp"})
	if err != nil {
		t.Fatal(err)
	}
	got := client.RawURL("feed_items/43/uuid.jpg")
	want := "https://img.fireimg.com/raw-images/needapp/feed_items/43/uuid.jpg"
	if got != want {
		t.Fatalf("%s want %s", got, want)
	}
}

func TestSrcSet(t *testing.T) {
	client, err := New(Config{Project: "needapp"})
	if err != nil {
		t.Fatal(err)
	}
	got := client.SrcSet("hero.jpg", SrcSetOptions{
		Widths:  []int{320, 640, 960},
		Options: Options{Quality: "high", Format: "auto"},
	})
	want := "https://img.fireimg.com/needapp/images/hero.jpg?width=320&quality=high&format=auto 320w, " +
		"https://img.fireimg.com/needapp/images/hero.jpg?width=640&quality=high&format=auto 640w, " +
		"https://img.fireimg.com/needapp/images/hero.jpg?width=960&quality=high&format=auto 960w"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}

	snapped := client.SrcSet("hero.jpg", SrcSetOptions{MinWidth: 400, MaxWidth: 800, SnapStep: 200})
	if snapped != "https://img.fireimg.com/needapp/images/hero.jpg?width=400 400w, https://img.fireimg.com/needapp/images/hero.jpg?width=600 600w, https://img.fireimg.com/needapp/images/hero.jpg?width=800 800w" {
		t.Fatalf("snap %s", snapped)
	}
}

func TestSrcSetFromURLAndSizes(t *testing.T) {
	src := "https://img.fireimg.com/needapp/images/hero.jpg?width=800&quality=high&format=auto"
	got := SrcSetFromURL(src, SrcSetOptions{Widths: []int{400, 800}})
	if got != "https://img.fireimg.com/needapp/images/hero.jpg?width=400&quality=high&format=auto 400w, https://img.fireimg.com/needapp/images/hero.jpg?width=800&quality=high&format=auto 800w" {
		t.Fatalf("%s", got)
	}
	if SizesAttr(800) != "(max-width: 800px) 100vw, 800px" {
		t.Fatalf("sizes %s", SizesAttr(800))
	}
	if SizesAttr(0) != "" {
		t.Fatal("empty sizes")
	}
}

func TestRewriteURL(t *testing.T) {
	src := "https://img.fireimg.com/needapp/images/hero.jpg?width=800&quality=high&format=auto"
	got := RewriteURL(src, Options{Width: 960})
	if got != "https://img.fireimg.com/needapp/images/hero.jpg?width=960&quality=high&format=auto" {
		t.Fatalf("%s", got)
	}
	if RewriteURL("not a url", Options{Width: 10}) != "not a url" {
		t.Fatal("invalid url should pass through")
	}
}

func TestIsURL(t *testing.T) {
	if !IsURL("https://img.fireimg.com/needapp/images/hero.jpg") {
		t.Fatal("canonical")
	}
	if !IsURL("https://i.fireimg.com/needapp/images/hero.jpg") {
		t.Fatal("alias")
	}
	if IsURL("https://example.com/hero.jpg") || IsURL("hero.jpg") {
		t.Fatal("non-fireimg")
	}
}

func TestSizesForWidths(t *testing.T) {
	sizes := SizesForWidths([]int{320, 0, 960})
	if len(sizes) != 2 || sizes[0].Width != 320 || sizes[1].Width != 960 {
		t.Fatalf("%+v", sizes)
	}
}
