package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	s3client "github.com/beravelli/s3uploader/internal/s3"
)

type FilesHandler struct {
	S3 *s3client.Client
}

func (h *FilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	files, err := h.S3.ListFiles(r.Context())
	if err != nil {
		log.Printf("listing files: %v", err)
		writeError(w, http.StatusBadGateway, "failed to list files")
		return
	}
	if files == nil {
		files = []s3client.FileInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
