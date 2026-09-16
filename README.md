# fireimg (Go)

Server-side Go client for API-key uploads, optional moderation, listing, and CDN URLs. It handles the two-step presigned upload (request URLs, then `PUT` to S3) the same way the [FireImg CLI](https://docs.fireimg.com/cli) and [Ruby gem](https://github.com/FirelitStudio/fireimg-ruby) do.

```go
client, err := fireimg.New(fireimg.Config{
    APIKey:  os.Getenv("FIREIMG_API_KEY"),
    Project: os.Getenv("FIREIMG_PROJECT"),
})

result, err := client.Upload(ctx, fireimg.Upload{
    Filename:    "hero.jpg",
    ContentType: "image/jpeg",
    Path:        "./hero.jpg",
})
// result.CDNURL => https://img.fireimg.com/your-project/images/hero.jpg
```

Create an API key under **API Keys** in the [FireImg app](https://app.fireimg.com/api-keys). The full secret is shown only once.

## Install

```bash
go get github.com/FirelitStudio/fireimg-go
```

```go
import "github.com/FirelitStudio/fireimg-go"
```

## Configure

```go
client, err := fireimg.New(fireimg.Config{
    APIKey:  os.Getenv("FIREIMG_API_KEY"),
    Project: os.Getenv("FIREIMG_PROJECT"),
    APIBase: os.Getenv("FIREIMG_API_BASE"), // optional; default https://api.fireimg.com
    CDNBase: os.Getenv("FIREIMG_CDN_BASE"), // optional; default https://img.fireimg.com
})
```

`FIREIMG_API_KEY` and `FIREIMG_PROJECT` (and the base URL variables) are also read automatically when those fields are empty. Prefer environment variables so the key does not appear in source or process lists.

Staging:

```go
client, err := fireimg.New(fireimg.Config{
    APIBase: "https://api.fireimg.dev",
    CDNBase: "https://img.fireimg.dev",
})
```

URL helpers work without an API key. Upload, delete, moderate, and list require both `APIKey` and `Project`.

## Upload

`Upload` requests a presigned URL, then `PUT`s the bytes:

```go
result, err := client.Upload(ctx, fireimg.Upload{
    Filename:    "feed_items/43/uuid.jpg",
    ContentType: "image/jpeg",
    Path:        "/tmp/photo.jpg", // or Body / Bytes
    Sizes: []fireimg.Size{
        {Width: 800, Quality: "medium", Fmt: "webp"},
        {Width: 160, Height: 160, Fit: "cover", Pos: "center", Fmt: "webp"},
    },
})

result.ImageKey // "feed_items/43/uuid.jpg"
result.CDNURL   // https://img.fireimg.com/{project}/images/feed_items/43/uuid.jpg
```

`Filename` may include folders. Provide the bytes with `Path`, `Body`, or `Bytes`.

Optional `Sizes` (up to 20) are generated asynchronously after the `PUT` and **replace** the project's default image params for that upload. Omit `Sizes` (nil) to keep project defaults. Pass `Sizes: []fireimg.Size{}` to pre-generate only FireImg's 80px dashboard thumbnail.

`UploadMany` batches the same way as the CLI (up to 20 files per presign request). Shared sizes come from the first file that sets them.

```go
result, err := client.Upload(ctx, fireimg.Upload{
    Filename:    "posts/kyoto/hero.jpg",
    ContentType: "image/jpeg",
    Body:        bytes.NewReader(data),
    Sizes:       fireimg.SizesForWidths([]int{320, 480, 640, 768, 960, 1280}),
})
```

CDN transforms are on-demand after the `PUT`. The client does not wait for optimization jobs.

If the client PUTs directly (mobile or browser), use `Presign` / `PresignMany` and `PUT` to `result.UploadURL` with the same `Content-Type`. See the [API reference](https://docs.fireimg.com/api/).

## Moderate, delete, and list

```go
labels, err := client.Moderate(ctx, "feed_items/43/uuid.jpg")
err = client.Delete(ctx, "feed_items/43/uuid.jpg")

projects, err := client.ListProjects(ctx)
page, err := client.ListImages(ctx, fireimg.ListImagesParams{Limit: 100, Folder: "feed_items"})
```

Moderation returns FireImg labels; your app applies policy.

## URLs

These helpers work with only `Project` set (no API key):

```go
client.PublicURL("feed_items/43/uuid.jpg")
// https://img.fireimg.com/{project}/images/feed_items/43/uuid.jpg

client.URL("feed_items/43/uuid.jpg", fireimg.Options{Width: 800, Quality: "medium", Format: "webp"})
// …/images/feed_items/43/uuid.jpg?width=800&quality=medium&format=webp

client.RawURL("feed_items/43/uuid.jpg")
// https://img.fireimg.com/raw-images/{project}/feed_items/43/uuid.jpg

srcset := client.SrcSet("hero.jpg", fireimg.SrcSetOptions{
    Widths:  []int{320, 640, 960},
    Options: fireimg.Options{Quality: "high", Format: "auto"},
})
// or snap a range: MinWidth, MaxWidth, SnapStep

sizes := fireimg.SizesAttr(960) // (max-width: 960px) 100vw, 960px
```

Supported options match the [Image URL reference](https://docs.fireimg.com/image-url): `Width`, `Height`, `Quality`, `Format` / `Fmt`, `Fit`, `Pos` / `Position`, `Fill`, and `Version`. Only set fields are emitted; quality is not defaulted.

To change the width on an existing FireImg URL:

```go
src := fireimg.RewriteURL(existing, fireimg.Options{Width: 960})
srcset := fireimg.SrcSetFromURL(existing, fireimg.SrcSetOptions{Widths: []int{320, 640, 960}})
```

`GuessContentType` and `SanitizeFilename` are available for upload prep.

## CLI

For uploads from a terminal, CI job, or coding agent, use the [FireImg CLI](https://docs.fireimg.com/cli) instead.

## License

MIT
