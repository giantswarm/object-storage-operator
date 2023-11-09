package objectstorage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	cloudaws "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud/aws"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewIAMService(ctx context.Context, logger logr.Logger, arn string, managementCluster managementcluster.ManagementCluster) (AccessRoleService, error)
	NewS3Service(ctx context.Context, logger logr.Logger, arn string, managementCluster managementcluster.ManagementCluster) (ObjectStorageService, error)
}

type factory struct {
}

func New() ObjectStorageServiceFactory {
	return factory{}
}

func (c factory) NewIAMService(ctx context.Context, logger logr.Logger, roleToAssume string, managementCluster managementcluster.ManagementCluster) (AccessRoleService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(managementCluster.Region))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, roleToAssume)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	parsedRole, err := arn.Parse(roleToAssume)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cloudaws.NewIamService(iam.NewFromConfig(cfg), logger, parsedRole.AccountID, managementCluster), nil
}

func (c factory) NewS3Service(ctx context.Context, logger logr.Logger, roleToAssume string, managementCluster managementcluster.ManagementCluster) (ObjectStorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(managementCluster.Region))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, roleToAssume)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	return cloudaws.NewS3Service(s3.NewFromConfig(cfg), logger, managementCluster.Region), nil
}
