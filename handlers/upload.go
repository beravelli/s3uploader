package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/beravelli/s3uploader/internal/photo"
	s3client "github.com/beravelli/s3uploader/internal/s3"
)

type UploadResponse struct {
	Key          string          `json:"key"`
	URL          string          `json:"url,omitempty"`
	StorageClass string          `json:"storage_class"`
	SizeBytes    int64           `json:"size_bytes"`
	EXIF         *photo.EXIFData `json:"exif"`
	Colors       []photo.Color   `json:"colors"`
}

type UploadHandler struct {
	S3      *s3client.Client
	MaxSize int64
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large or malformed form")
		return
	}

	storageClassName := r.FormValue("storage_class")
	storageClass, ok := s3client.StorageClassMap[storageClassName]
	if !ok {
		writeError(w, http.StatusBadRequest, "storage_class must be STANDARD or DEEP_ARCHIVE")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reading upload")
		return
	}

	contentType := http.DetectContentType(data[:min(len(data), 512)])
	if !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusUnsupportedMediaType, fmt.Sprintf("unsupported file type: %s", contentType))
		return
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("%d/%02d/%s-%s", now.Year(), now.Month(), uuid.NewString()[:8], sanitizeFilename(header.Filename))

	var exifData *photo.EXIFData
	var colors []photo.Color
	g, ctx := errgroup.WithContext(r.Context())
	g.Go(func() error {
		exifData, _ = photo.ExtractEXIF(bytes.NewReader(data))
		return nil
	})
	g.Go(func() error {
		c, err := photo.ExtractPalette(bytes.NewReader(data), 5)
		if err != nil {
			log.Printf("color extraction failed for %s: %v", key, err)
			c = []photo.Color{}
		}
		colors = c
		return nil
	})
	g.Go(func() error {
		return h.S3.Upload(ctx, key, bytes.NewReader(data), contentType, storageClass, map[string]string{
			"original-filename": header.Filename,
			"uploaded-at":       now.Format(time.RFC3339),
		})
	})
	if err := g.Wait(); err != nil {
		log.Printf("upload failed: %v", err)
		writeError(w, http.StatusBadGateway, "S3 upload failed")
		return
	}

	resp := UploadResponse{
		Key:          key,
		StorageClass: storageClassName,
		SizeBytes:    int64(len(data)),
		EXIF:         exifData,
		Colors:       colors,
	}
	if storageClassName == "STANDARD" {
		if url, err := h.S3.PresignGet(context.Background(), key); err == nil {
			resp.URL = url
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
