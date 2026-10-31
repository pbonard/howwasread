package common

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CDNClient hands out time limited garage URLs for uploaded media
type CDNClient interface {
	SignedURL(contentType, filename string) (string, error)
}

type cdnClient struct {
	presignClient *s3.PresignClient
	bucketName    string
}

// NewPresignClient signs urls for S3_PUBLIC_URL, the garage address the app reaches through cloudflare.
// Signing never calls garage, so the in cluster AWS_ENDPOINT_URL is left unused.
// Keys, region and bucket come from the garage-s3 secret.
func NewPresignClient() *s3.PresignClient {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Panicf("fail to config for presigned client: %v", err)
	}
	return s3.NewPresignClient(s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("S3_PUBLIC_URL"))
		// garage serves the bucket as a path, s3.pbonard.com/<bucket>/<key>
		o.UsePathStyle = true
	}))
}

func NewCDNClient() CDNClient {
	return &cdnClient{
		presignClient: NewPresignClient(),
		bucketName:    os.Getenv("S3_BUCKET"),
	}
}

// SignedURL presigns a GET of <content type>/<filename> for an hour
func (c *cdnClient) SignedURL(contentType, filename string) (string, error) {
	p, err := c.presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(fmt.Sprintf("%s/%s", contentType, filename)),
	}, s3.WithPresignExpires(1*time.Hour))
	if err != nil {
		slog.Error("fail to generate signed URL", "err", err)
		return "", err
	}
	return p.URL, nil
}
