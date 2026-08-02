package objectstore

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/usenorn/norn/internal/config"
)

const credentialsSource = "norn-config"

var ErrIncompleteConfiguration = errors.New("object storage configuration is incomplete")

type Client struct {
	*s3.Client

	bucket        string
	endpoint      string
	publicBaseURL string
}

func New(cfg config.Storage) (*Client, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, ErrIncompleteConfiguration
	}

	provider := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			Source:          credentialsSource,
		}, nil
	})

	awsConfig := aws.Config{
		Region:      cfg.Region,
		Credentials: aws.NewCredentialsCache(provider),
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Client{
		Client:        client,
		bucket:        cfg.Bucket,
		endpoint:      cfg.Endpoint,
		publicBaseURL: cfg.PublicBaseURL,
	}, nil
}

func (c *Client) Bucket() string {
	return c.bucket
}

func (c *Client) PublicBaseURL() string {
	if c.publicBaseURL != "" {
		return c.publicBaseURL
	}

	return strings.TrimSuffix(c.endpoint, "/") + "/" + c.bucket
}
