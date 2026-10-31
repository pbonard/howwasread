package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"backend/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StorageClient lets clients upload media straight to garage
type StorageClient interface {
	// PresignUpload returns the form url and fields of a presigned POST for <contentType>/<filename>
	PresignUpload(ctx context.Context, contentType, filename string) (url string, fields map[string]string, err error)
}

type storageClient struct {
	presignClient *s3.PresignClient
	bucketName    string
}

func NewStorageClient() StorageClient {
	return &storageClient{
		presignClient: common.NewPresignClient(),
		bucketName:    os.Getenv("S3_BUCKET"),
	}
}

func (s *storageClient) PresignUpload(ctx context.Context, contentType, filename string) (string, map[string]string, error) {
	p, err := s.presignClient.PresignPostObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(fmt.Sprintf("%s/%s", contentType, filename)),
	}, func(opts *s3.PresignPostOptions) {
		opts.Expires = 1 * time.Hour
		opts.Conditions = []any{
			[]any{"content-length-range", 1, 1024 * 1024 * 1024},
			[]any{"starts-with", "$Content-Type", contentType},
		}
	})
	if err != nil {
		return "", nil, err
	}
	return p.URL, p.Values, nil
}
