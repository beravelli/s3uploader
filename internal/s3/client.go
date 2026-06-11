package s3client

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/beravelli/s3uploader/internal/config"
)

var StorageClassMap = map[string]types.StorageClass{
	"STANDARD":     types.StorageClassStandard,
	"DEEP_ARCHIVE": types.StorageClassDeepArchive,
}

type Client struct {
	s3        *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

type FileInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	StorageClass string    `json:"storage_class"`
	URL          string    `json:"url,omitempty"`
	Retrievable  bool      `json:"retrievable"`
}

func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	c := s3.NewFromConfig(awsCfg)
	return &Client{
		s3:        c,
		presigner: s3.NewPresignClient(c),
		bucket:    cfg.S3Bucket,
	}, nil
}

func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string, storageClass types.StorageClass, metadata map[string]string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       &c.bucket,
		Key:          &key,
		Body:         body,
		ContentType:  &contentType,
		StorageClass: storageClass,
		Metadata:     metadata,
	})
	if err != nil {
		return fmt.Errorf("putting object %s: %w", key, err)
	}
	return nil
}

func (c *Client) PresignGet(ctx context.Context, key string) (string, error) {
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return "", fmt.Errorf("presigning %s: %w", key, err)
	}
	return req.URL, nil
}

func (c *Client) ListFiles(ctx context.Context) ([]FileInfo, error) {
	var files []FileInfo
	paginator := s3.NewListObjectsV2Paginator(c.s3, &s3.ListObjectsV2Input{Bucket: &c.bucket})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing bucket %s: %w", c.bucket, err)
		}
		for _, obj := range page.Contents {
			sc := string(obj.StorageClass)
			if sc == "" {
				sc = "STANDARD"
			}
			fi := FileInfo{
				Key:          *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				StorageClass: sc,
				Retrievable:  sc != string(types.StorageClassDeepArchive) && sc != string(types.StorageClassGlacier),
			}
			if fi.Retrievable {
				if url, err := c.PresignGet(ctx, fi.Key); err == nil {
					fi.URL = url
				}
			}
			files = append(files, fi)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})
	return files, nil
}
