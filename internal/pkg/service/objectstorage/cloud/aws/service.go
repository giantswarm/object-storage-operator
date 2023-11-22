package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type AWSObjectStorageService struct {
}

func (s AWSObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cluster.GetRegion()))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, cluster.GetRole())
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	parsedRole, err := arn.Parse(cluster.GetRole())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewIamService(iam.NewFromConfig(cfg), logger, parsedRole.AccountID, cluster.GetBaseDomain(), cluster.GetName()), nil
}

func (s AWSObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.ObjectStorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cluster.GetRegion()))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, cluster.GetRole())
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	return NewS3Service(s3.NewFromConfig(cfg), logger, cluster.GetRegion()), nil
}
