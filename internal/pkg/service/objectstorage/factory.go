package objectstorage

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	cloudaws "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud/aws"
	cloudazure "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud/azure"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewIAMService(ctx context.Context, logger logr.Logger, arn string, managementCluster managementcluster.ManagementCluster) (AccessRoleService, error)
	NewS3Service(ctx context.Context, logger logr.Logger, arn string, managementCluster managementcluster.ManagementCluster) (ObjectStorageService, error)
	NewAzureStorageService(ctx context.Context, logger logr.Logger, cli client.Client, azCluster cloudazure.AzureCluster) (ObjectStorageService, error)
}

type factory struct {
}

const (
	clientSecretKeyName = "clientSecret"
)

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

func (c factory) NewAzureStorageService(ctx context.Context, logger logr.Logger, cli client.Client, azCluster cloudazure.AzureCluster) (ObjectStorageService, error) {
	var cred azcore.TokenCredential
	var err error
	var storageClientFactory *armstorage.ClientFactory

	switch azCluster.AzureIdentity.Type {
	case "UserAssignedMSI":
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(azCluster.AzureIdentity.ClientID),
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	case "ManualServicePrincipal":
		clientSecretName := types.NamespacedName{
			Namespace: azCluster.AzureIdentity.ClientSecret.Namespace,
			Name:      azCluster.AzureIdentity.ClientSecret.Name,
		}
		secret := &corev1.Secret{}
		err = cli.Get(ctx, clientSecretName, secret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		cred, err = azidentity.NewClientSecretCredential(
			azCluster.AzureIdentity.TenantID,
			azCluster.AzureIdentity.ClientID,
			string(secret.Data[clientSecretKeyName]),
			nil)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	storageClientFactory, err = armstorage.NewClientFactory(azCluster.SubscriptionID, cred, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return cloudazure.NewAzureStorageService(storageClientFactory, logger), nil
}
