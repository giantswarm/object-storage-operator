package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage/v3"
	"github.com/go-logr/logr"
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
		return nil, fmt.Errorf("failed to cast cluster credentials to Azure credentials for cluster %s", cluster.GetName())
	}
	switch azureCredentials.TypeIdentity {
	case "UserAssignedMSI":
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(azureCredentials.ClientID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create managed identity credential for cluster %s with client ID %s: %w", cluster.GetName(), azureCredentials.ClientID, err)
		}
	case "ManualServicePrincipal":
		cred, err = azidentity.NewClientSecretCredential(
			azureCredentials.TenantID,
			azureCredentials.ClientID,
			string(azureCredentials.SecretRef.Data[ClientSecretKeyName]),
			nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client secret credential for cluster %s with tenant ID %s: %w", cluster.GetName(), azureCredentials.TenantID, err)
		}
	case "WorkloadIdentity":
		cred, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			TenantID: azureCredentials.TenantID,
			ClientID: azureCredentials.ClientID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create workload identity credential for cluster %s with tenant ID %s: %w", cluster.GetName(), azureCredentials.TenantID, err)
		}
	default:
		return nil, fmt.Errorf("unknown identity type %s for cluster %s", azureCredentials.TypeIdentity, cluster.GetName())
	}

	var storageClientFactory *armstorage.ClientFactory
	storageClientFactory, err = armstorage.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client factory for cluster %s with subscription ID %s: %w", cluster.GetName(), azureCredentials.SubscriptionID, err)
	}

	var networkClientFactory *armnetwork.ClientFactory
	networkClientFactory, err = armnetwork.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network client factory for cluster %s with subscription ID %s: %w", cluster.GetName(), azureCredentials.SubscriptionID, err)
	}

	var privateZonesClientFactory *armprivatedns.ClientFactory
	privateZonesClientFactory, err = armprivatedns.NewClientFactory(azureCredentials.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create private DNS client factory for cluster %s with subscription ID %s: %w", cluster.GetName(), azureCredentials.SubscriptionID, err)
	}

	azurecluster, ok := cluster.(AzureCluster)
	if !ok {
		return nil, fmt.Errorf("failed to cast cluster to Azure cluster for cluster %s", cluster.GetName())
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
