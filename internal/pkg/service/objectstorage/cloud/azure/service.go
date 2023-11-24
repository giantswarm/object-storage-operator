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
	//TODO
	return nil, nil
}

func (s AzureObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.ObjectStorageService, error) {
	var cred azcore.TokenCredential
	var err error

	switch cluster.GetTypeIdentity() {
	case "UserAssignedMSI":
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(cluster.GetClientID()),
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
	case "ManualServicePrincipal":
		cred, err = azidentity.NewClientSecretCredential(
			cluster.GetTenantID(),
			cluster.GetClientID(),
			string(cluster.GetSecretRef().Data[ClientSecretKeyName]),
			nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	default:
		return nil, errors.New(fmt.Sprintf("Unknown typeIdentity %s", cluster.GetTypeIdentity()))
	}

	var storageClientFactory *armstorage.ClientFactory
	storageClientFactory, err = armstorage.NewClientFactory(cluster.GetSubscriptionID(), cred, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewAzureStorageService(storageClientFactory.NewAccountsClient(), storageClientFactory.NewBlobContainersClient(), logger, cluster.GetResourceGroup()), nil
}
