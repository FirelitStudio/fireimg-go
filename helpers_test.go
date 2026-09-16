package fireimg

import "testing"

func TestGuessContentType(t *testing.T) {
	tests := []struct {
		filename string
		header   string
		want     string
	}{
		{"photo.PNG", "", "image/png"},
		{"photo.gif", "", "image/gif"},
		{"photo.webp", "", "image/webp"},
		{"photo.avif", "", "image/avif"},
		{"logo.svg", "", "image/svg+xml"},
		{"scan.tiff", "", "image/tiff"},
		{"shot.heic", "", "image/heic"},
		{"photo.jpg", "", "image/jpeg"},
		{"photo.jpg", "image/png; charset=binary", "image/png"},
		{"photo.jpg", "application/octet-stream", "image/jpeg"},
	}
	for _, tt := range tests {
		if got := GuessContentType(tt.filename, tt.header); got != tt.want {
			t.Fatalf("%s %q => %s want %s", tt.filename, tt.header, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hero.jpg", "hero.jpg"},
		{"feed_items/43/uuid.jpg", "feed_items/43/uuid.jpg"},
		{"weird name!.jpg", "weird-name.jpg"},
		{"/etc/passwd", "passwd"},
		{"album/../secret.jpg", "album/secret.jpg"},
		{"", "image"},
		{"...", "image"},
	}
	for _, tt := range tests {
		if got := SanitizeFilename(tt.in); got != tt.want {
			t.Fatalf("%q => %q want %q", tt.in, got, tt.want)
		}
	}
}
