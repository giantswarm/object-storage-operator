package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

type AzureObjectStorageService struct {
}

func (s AzureObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	return NewAzureAccessService(), nil
}

func (s AzureObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.ObjectStorageService, error) {
	var cred azcore.TokenCredential
	var err error

	azureCredentials, ok := cluster.GetCredentials().(AzureCredentials)
	if !ok {
		return nil, errors.New("Impossible to cast cluster credentials into Azure cluster credentials")
	}
	switch azureCredentials.TypeIdentity {
	case "UserAssignedMSI":
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(azureCredentials.ClientID),
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case "ManualServicePrincipal":
		cred, err = azidentity.NewClientSecretCredential(
			azureCredentials.TenantID,
			azureCredentials.ClientID,
			string(azureCredentials.SecretRef.Data[ClientSecretKeyName]),
			nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	default:
		return nil, errors.New(fmt.Sprintf("Unknown typeIdentity %s", azureCredentials.TypeIdentity))
	}

	var storageClientFactory *armstorage.ClientFactory
	storageClientFactory, err = armstorage.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewAzureStorageService(storageClientFactory.NewAccountsClient(), storageClientFactory.NewBlobContainersClient(), logger, cluster.(AzureCluster)), nil
}
