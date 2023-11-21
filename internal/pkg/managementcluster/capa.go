package managementcluster

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	objectstorage "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
	cloudaws "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud/aws"
)

type CAPACluster struct {
	ManagementCluster ManagementCluster
	RoleToAssume      string
}

func (c CAPACluster) NewAccessRoleService(ctx context.Context, logger logr.Logger) (objectstorage.AccessRoleService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.ManagementCluster.Region))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, c.RoleToAssume)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	parsedRole, err := arn.Parse(c.RoleToAssume)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cloudaws.NewIamService(iam.NewFromConfig(cfg), logger, parsedRole.AccountID, c.ManagementCluster.BaseDomain, c.ManagementCluster.Name), nil
}

func (c CAPACluster) NewObjectStorageService(ctx context.Context, logger logr.Logger) (objectstorage.ObjectStorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.ManagementCluster.Region))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, c.RoleToAssume)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	return cloudaws.NewS3Service(s3.NewFromConfig(cfg), logger, c.ManagementCluster.Region), nil
}

func (c CAPACluster) GetRoleArn(ctx context.Context, req ctrl.Request, cli client.Client) (string, error) {
	logger := log.FromContext(ctx)

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Kind:    "AWSCluster",
		Version: "v1beta2",
	})

	err := cli.Get(ctx, client.ObjectKey{
		Name:      c.ManagementCluster.Name,
		Namespace: c.ManagementCluster.Namespace,
	}, cluster)
	if err != nil {
		logger.Error(err, "Missing management cluster AWSCluster CR")
		return "", errors.WithStack(err)
	}

	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return "", errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return "", errors.New("missing management cluster identify")
	}

	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Kind:    "AWSClusterRoleIdentity",
		Version: "v1beta2",
	})

	err = cli.Get(ctx, client.ObjectKey{
		Name:      clusterIdentityName,
		Namespace: cluster.GetNamespace(),
	}, clusterIdentity)
	if err != nil {
		logger.Error(err, "Missing management cluster identity AWSClusterRoleIdentity CR")
		return "", errors.WithStack(err)
	}

	roleArn, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "roleARN")
	if err != nil {
		logger.Error(err, "Role arn is not a string")
		return "", errors.WithStack(err)
	}
	if !found {
		return "", errors.New("missing role arn")
	}
	return roleArn, nil
}
