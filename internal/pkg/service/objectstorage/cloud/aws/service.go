package aws

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

type AWSObjectStorageService struct {
}

func (s AWSObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cluster.GetRegion()))
	if err != nil {
		return nil, err
	}

	// Assume role
	awsCredentials, ok := cluster.GetCredentials().(AWSCredentials)
	if !ok {
		return nil, errors.New("Impossible to cast cluster credentials into AWS cluster credentials")
	}
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, awsCredentials.Role)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	parsedRole, err := arn.Parse(awsCredentials.Role)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	awscluster, ok := cluster.(AWSCluster)
	if !ok {
		return nil, errors.New("Impossible to cast cluster into AWS cluster")
	}
	return NewIamService(iam.NewFromConfig(cfg), logger, parsedRole.AccountID, awscluster), nil
}

func (s AWSObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, client client.Client) (objectstorage.ObjectStorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cluster.GetRegion()))
	if err != nil {
		return nil, err
	}

	// Assume role
	awsCredentials, ok := cluster.GetCredentials().(AWSCredentials)
	if !ok {
		return nil, errors.New("Impossible to cast cluster credentials into AWS cluster credentials")
	}
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, awsCredentials.Role)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	awscluster, ok := cluster.(AWSCluster)
	if !ok {
		return nil, errors.New("Impossible to cast cluster into AWS cluster")
	}
	return NewS3Service(s3.NewFromConfig(cfg), logger, awscluster), nil
}
