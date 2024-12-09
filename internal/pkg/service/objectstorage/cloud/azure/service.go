package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

type AzureObjectStorageService struct {
}

func (s AzureObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	return NewAzureAccessService(), nil
}

func (s AzureObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, client client.Client) (objectstorage.ObjectStorageService, error) {
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

	var networkClientFactory *armnetwork.ClientFactory
	networkClientFactory, err = armnetwork.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var privateZonesClientFactory *armprivatedns.ClientFactory
	privateZonesClientFactory, err = armprivatedns.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	azurecluster, ok := cluster.(AzureCluster)
	if !ok {
		return nil, errors.New("Impossible to cast cluster into Azure cluster")
	}
	return NewAzureStorageService(
		storageClientFactory.NewAccountsClient(),
		storageClientFactory.NewBlobContainersClient(),
		storageClientFactory.NewManagementPoliciesClient(),
		networkClientFactory.NewPrivateEndpointsClient(),
		privateZonesClientFactory.NewPrivateZonesClient(),
		privateZonesClientFactory.NewRecordSetsClient(),
		privateZonesClientFactory.NewVirtualNetworkLinksClient(),
		logger,
		azurecluster,
		client,
	), nil
}
