package objectstorage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewS3Service(ctx context.Context, region, arn string) (ObjectStorageService, error)
}

type factory struct {
}

func New() ObjectStorageServiceFactory {
	return factory{}
}

func (c factory) NewS3Service(ctx context.Context, region, roleToAssume string) (ObjectStorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, roleToAssume)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	return cloud.NewS3Service(s3.NewFromConfig(cfg), region), nil
}
