# S3 Photo Uploader — Web App

## Context
Build a Go web app that lets a user upload photos to AWS S3 with a configurable storage class (Standard or Glacier Deep Archive), and immediately displays rich metadata extracted from the photo: EXIF camera info, a dominant color palette, and an interactive GPS map if coordinates are embedded.

---

## Stack
- **Backend**: Go `net/http` (no framework), AWS SDK v2 for S3
- **Frontend**: Vanilla HTML/CSS/JS, Leaflet.js (CDN) for the map
- **EXIF**: `github.com/rwcarlsen/goexif/exif`
- **Color extraction**: Pure Go median-cut (no extra dep), using `golang.org/x/image/draw` to resize
- **Config**: `.env` file or environment variables

---

## Directory Structure

```
s3uploader/
├── main.go
├── go.mod
├── .env.example
├── handlers/
│   ├── upload.go       # POST /api/upload
│   └── files.go        # GET /api/files
├── internal/
│   ├── config/config.go
│   ├── s3/client.go
│   └── photo/
│       ├── exif.go
│       └── colors.go
└── static/
    ├── index.html
    ├── css/style.css
    └── js/
        ├── upload.js
        ├── metadata.js
        ├── map.js
        └── files.js
```

---

## API

| Method | Path | Description |
|---|---|---|
| `GET /` | — | Serve `static/index.html` |
| `POST /api/upload` | `multipart: file, storage_class` | Upload photo, return metadata JSON |
| `GET /api/files` | — | List uploaded files from S3 |

### POST /api/upload — Response shape
```json
{
  "key": "2026/06/uuid-photo.jpg",
  "url": "https://bucket.s3.amazonaws.com/...",
  "storage_class": "DEEP_ARCHIVE",
  "exif": {
    "camera_model": "Canon EOS R5",
    "aperture": "f/2.8",
    "shutter_speed": "1/500s",
    "iso": 400,
    "focal_length": "85mm",
    "date_taken": "2024-03-15T14:22:00Z",
    "gps_lat": 48.8566,
    "gps_lng": 2.3522
  },
  "colors": [
    { "hex": "#3a7bd5" },
    { "hex": "#f5a623" },
    { "hex": "#d0021b" },
    { "hex": "#417505" },
    { "hex": "#9b9b9b" }
  ]
}
```

---

## Implementation Details

### Upload handler flow (`handlers/upload.go`)
1. Wrap body with `http.MaxBytesReader` for size enforcement (413)
2. `ParseMultipartForm`, read `storage_class` field, validate it is `STANDARD` or `DEEP_ARCHIVE`
3. `FormFile("file")`, detect MIME from first 512 bytes — reject non-image with 415
4. Read entire body into `[]byte` (needed for both EXIF and color extraction to seek)
5. Generate S3 key: `<year>/<month>/<uuid>-<original-filename>`
6. Run EXIF extraction and color extraction **concurrently** with `errgroup`
7. Upload to S3 with chosen `StorageClass`
8. Return `UploadResponse` JSON

### Storage class mapping (`internal/s3/client.go`)
```go
var storageClassMap = map[string]types.StorageClass{
    "STANDARD":     types.StorageClassStandard,
    "DEEP_ARCHIVE": types.StorageClassDeepArchive,
}
```
Files listed with `DEEP_ARCHIVE` storage class get a `retrievable: false` flag — JS renders a badge instead of a download link.

### EXIF extraction (`internal/photo/exif.go`)
- Use `goexif` `x.Get(tag)` for each field, catch individual errors (missing tag ≠ failure)
- Format: aperture as `f/2.8`, shutter as `1/200s`, focal length as `50mm`
- GPS via `x.LatLong()` — only populate pointer fields if this returns no error
- If `exif.Decode` itself fails (no EXIF block), return empty struct + nil error

### Color extraction (`internal/photo/colors.go`)
1. Decode image, resize to 50×50 using `golang.org/x/image/draw` CatmullRom
2. Sample all 2500 pixels (skip transparent)
3. Median-cut algorithm (~60 lines pure Go): repeatedly split the widest color-range bucket until 5 buckets remain, average each bucket
4. Return 5 `Color{Hex: "#rrggbb"}` sorted by luminance

### Frontend (`static/`)
- **Drag-drop zone**: `dragover` + `drop` events, fallback file picker
- **Preview**: `FileReader` → `<img src>` shown immediately before upload completes
- **Upload progress**: `xhr.upload.addEventListener('progress')` → progress bar + percentage
- **Storage class select**: `<select>` with Standard / Glacier Deep Archive options
- **EXIF table**: rendered from response JSON, fields skipped if empty
- **Color swatches**: `<div>` elements with `background: #hex`, hover shows hex value
- **GPS map**: Leaflet.js on OpenStreetMap; `mapSection.hidden = true` if no GPS; destroy previous map instance before re-creating to avoid "already initialized" error
- **File list**: fetched from `GET /api/files`, Glacier files show archive badge instead of link

### Config (`.env.example`)
```
AWS_REGION=us-east-1
S3_BUCKET=my-photo-bucket
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
PORT=8080
MAX_UPLOAD_MB=100
```
Config loader: parse `.env` with `bufio.Scanner` as fallback if env vars not set.

---

## go.mod Dependencies
```
github.com/aws/aws-sdk-go-v2                v1.26.0
github.com/aws/aws-sdk-go-v2/config         v1.27.0
github.com/aws/aws-sdk-go-v2/credentials    v1.17.0
github.com/aws/aws-sdk-go-v2/service/s3     v1.53.0
github.com/rwcarlsen/goexif                  v0.0.0-20190401172101-9e8deecbddbd
golang.org/x/image                           v0.15.0
```

---

## Edge Cases
| Case | Handling |
|---|---|
| No EXIF (PNG, WebP, stripped JPEG) | `exif.Decode` error → return empty struct, no API error |
| No GPS in EXIF | `x.LatLong()` error → pointer fields nil, omitted from JSON, map section hidden |
| Non-image file | MIME detection → HTTP 415 before S3 call |
| File too large | `MaxBytesReader` → HTTP 413 |
| Glacier file listing | `retrievable: false` flag, download link replaced by badge |
| Leaflet re-init on second upload | Call `leafletMap.remove()` before re-creating |

---

## Verification
1. `go run .` starts server on `:8080`
2. Upload a JPEG with GPS data → verify EXIF table, map pin, and color swatches all render
3. Upload a PNG (no EXIF) → verify graceful empty EXIF section, map hidden, colors still show
4. Select Glacier Deep Archive → verify S3 object has `STORAGE-CLASS: GLACIER_IR` (check in AWS console or `aws s3api head-object`)
5. Check `/api/files` returns the uploaded file with correct storage class
6. Upload a non-image file → verify HTTP 415 response
