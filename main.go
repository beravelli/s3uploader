package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/beravelli/s3uploader/handlers"
	"github.com/beravelli/s3uploader/internal/config"
	s3client "github.com/beravelli/s3uploader/internal/s3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	s3c, err := s3client.NewClient(context.Background(), cfg)
	if err != nil {
		log.Fatalf("s3 client: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /api/upload", &handlers.UploadHandler{S3: s3c, MaxSize: cfg.MaxUploadBytes})
	mux.Handle("GET /api/files", &handlers.FilesHandler{S3: s3c})
	mux.Handle("GET /", http.FileServer(http.Dir("static")))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      logRequests(mux),
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
	}

	go func() {
		log.Printf("listening on http://localhost:%s (bucket: %s, region: %s)", cfg.Port, cfg.S3Bucket, cfg.AWSRegion)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
