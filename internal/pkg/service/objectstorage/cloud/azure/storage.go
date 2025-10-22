package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage/v3"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

const (
	LifecycleRuleName = "ExpirationLogging"
)

type AzureObjectStorageAdapter struct {
	storageAccountClient      *armstorage.AccountsClient
	blobContainerClient       *armstorage.BlobContainersClient
	managementPoliciesClient  *armstorage.ManagementPoliciesClient
	privateEndpointsClient    *armnetwork.PrivateEndpointsClient
	privateZonesClient        *armprivatedns.PrivateZonesClient
	recordSetsClient          *armprivatedns.RecordSetsClient
	virtualNetworkLinksClient *armprivatedns.VirtualNetworkLinksClient
	logger                    logr.Logger
	cluster                   AzureCluster
	client                    client.Client
}

// NewAzureStorageService creates a new instance of AzureObjectStorageAdapter.
// It takes in the necessary parameters to initialize the adapter and returns the created instance.
// The storageAccountClient, blobContainerClient, and managementPoliciesClient are clients for interacting with Azure storage resources.
// The logger is used for logging purposes.
// The cluster represents the Azure cluster.
// The client is the Kubernetes client used for interacting with the Kubernetes API.
func NewAzureStorageService(
	storageAccountClient *armstorage.AccountsClient,
	blobContainerClient *armstorage.BlobContainersClient,
	managementPoliciesClient *armstorage.ManagementPoliciesClient,
	privateEndpointsClient *armnetwork.PrivateEndpointsClient,
	privateZonesClient *armprivatedns.PrivateZonesClient,
	recordSetsClient *armprivatedns.RecordSetsClient,
	virtualNetworkLinksClient *armprivatedns.VirtualNetworkLinksClient,
	logger logr.Logger,
	cluster AzureCluster,
	client client.Client) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		storageAccountClient:      storageAccountClient,
		blobContainerClient:       blobContainerClient,
		managementPoliciesClient:  managementPoliciesClient,
		privateEndpointsClient:    privateEndpointsClient,
		privateZonesClient:        privateZonesClient,
		recordSetsClient:          recordSetsClient,
		virtualNetworkLinksClient: virtualNetworkLinksClient,
		logger:                    logger,
		cluster:                   cluster,
		client:                    client,
	}
}

// ExistsBucket checks if a bucket exists in the Azure Object Storage.
// It first checks if the storage account exists on Azure. If the storage account does not exist,
// it means the bucket does not exist either, so it returns false.
// If the storage account exists, it then checks if the BlobContainer with the specified name exists in the storage account.
// If the BlobContainer does not exist, it returns false. Otherwise, it returns true.
func (s AzureObjectStorageAdapter) ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)

	// Check if storage account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx, storageAccountName)
	if err != nil {
		return false, err
	}
	// If StorageAccount does not exists that means the bucket does not exists too, so we return false
	if !existsStorageAccount {
		return false, nil
	}

	return s.existsContainer(ctx, bucket, storageAccountName)
}

// isManagementClusterPrivate checks if the management cluster is private by reading the cluster user-values CM
func (s AzureObjectStorageAdapter) isManagementClusterPrivate(ctx context.Context) (bool, error) {
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-user-values", s.cluster.GetName()),
		Namespace: "org-giantswarm",
	}

	configMap := &v1.ConfigMap{}
	if err := s.client.Get(ctx, key, configMap); client.IgnoreNotFound(err) != nil {
		return false, err
	} else if apierrors.IsNotFound(err) {
		return false, nil
	}

	networkingConfig := struct {
		Global *struct {
			Connectivity *struct {
				Network *struct {
					Mode *string `yaml:"mode"`
				} `yaml:"network"`
			} `yaml:"connectivity"`
		} `yaml:"global"`
	}{}

	err := yaml.Unmarshal([]byte(configMap.Data["values"]), &networkingConfig)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal network config for cluster %s: %w", s.cluster.GetName(), err)
	}

	return networkingConfig.Global != nil &&
		networkingConfig.Global.Connectivity != nil &&
		networkingConfig.Global.Connectivity.Network != nil &&
		networkingConfig.Global.Connectivity.Network.Mode != nil &&
		*networkingConfig.Global.Connectivity.Network.Mode == "private", nil
}

// CreateBucket creates the Storage Account if it not exists AND the Storage Container
// It checks if the storage account exists, and if not, it creates it.
// Then, it creates a storage container within the storage account.
// Finally, it retrieves the access key for 'key1' and creates a K8S Secret to store the storage account access key.
// The Secret is created in the same namespace as the bucket.
// The function returns an error if any of the operations fail.
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)

	isPrivateManagementCluster, err := s.isManagementClusterPrivate(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if management cluster is private for bucket %s: %w", bucket.Spec.Name, err)
	}

	if err := s.upsertStorageAccount(ctx, bucket, storageAccountName, isPrivateManagementCluster); err != nil {
		return fmt.Errorf("failed to upsert storage account %s for bucket %s: %w", storageAccountName, bucket.Spec.Name, err)
	}

	if err := s.upsertContainer(ctx, bucket, storageAccountName); err != nil {
		return fmt.Errorf("failed to upsert container for bucket %s in storage account %s: %w", bucket.Spec.Name, storageAccountName, err)
	}

	if isPrivateManagementCluster {
		if _, err := s.upsertPrivateZone(ctx, bucket); err != nil {
			return fmt.Errorf("failed to upsert private zone for bucket %s: %w", bucket.Spec.Name, err)
		}

		if _, err := s.upsertVirtualNetworkLink(ctx, bucket); err != nil {
			return fmt.Errorf("failed to upsert virtual network link for bucket %s: %w", bucket.Spec.Name, err)
		}

		privateEndpoint, err := s.upsertPrivateEndpoint(ctx, bucket, storageAccountName)
		if err != nil {
			return fmt.Errorf("failed to upsert private endpoint for bucket %s in storage account %s: %w", bucket.Spec.Name, storageAccountName, err)
		}

		if _, err = s.upsertPrivateEndpointARecords(ctx, bucket, privateEndpoint, storageAccountName); err != nil {
			return fmt.Errorf("failed to upsert private endpoint A records for bucket %s: %w", bucket.Spec.Name, err)
		}
	}

	// Create a K8S Secret to store Storage Account Access Key
	// First, we retrieve Storage Account Access Key on Azure
	listKeys, err := s.storageAccountClient.ListKeys(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("unable to retrieve access keys from storage account %s", storageAccountName)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bucket.Spec.Name,
			Namespace: bucket.Namespace,
			Labels: map[string]string{
				"giantswarm.io/managed-by": "object-storage-operator",
			},
			Finalizers: []string{
				v1alpha1.AzureSecretFinalizer,
			},
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, s.client, &secret, func() error {
		// Then, we retrieve the Access Key for 'key1'
		for _, k := range listKeys.Keys {
			if *k.KeyName == "key1" {
				// Finally, we create the Secret into the bucket namespace
				secret.Data = map[string][]byte{
					"accountName": []byte(storageAccountName),
					"accountKey":  []byte(*k.Value),
				}
				return nil
			}
		}

		return fmt.Errorf("unable to retrieve access keys 'key1' from storage account %s", storageAccountName)
	})

	if err != nil {
		return fmt.Errorf("failed to create or update secret %s for bucket %s: %w", bucket.Spec.Name, bucket.Spec.Name, err)
	}

	s.logger.Info(fmt.Sprintf("upserted secret %s", bucket.Spec.Name))
	return nil
}

// UpdateBucket creates or updates the Storage Account AND the Storage Container
func (s AzureObjectStorageAdapter) UpdateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return s.CreateBucket(ctx, bucket)
}

// DeleteBucket deletes the Storage Account (Storage Container will be deleted by cascade)
// Here, we decided to have a Storage Account dedicated to a Storage Container (relation 1 - 1)
// We want to prevent the Storage Account from being used by anyone
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)

	if err := s.deleteStorageAccount(ctx, bucket, storageAccountName); err != nil {
		return fmt.Errorf("failed to delete storage account %s for bucket %s: %w", storageAccountName, bucket.Spec.Name, err)
	}

	// We delete the Azure Credentials secret
	var secret = v1.Secret{}
	err := s.client.Get(
		ctx,
		types.NamespacedName{
			Name:      bucket.Spec.Name,
			Namespace: bucket.Namespace,
		},
		&secret)
	if err != nil {
		return fmt.Errorf("failed to get secret %s for bucket %s: %w", bucket.Spec.Name, bucket.Spec.Name, err)
	}
	// We remove the finalizer to allow the secret to be deleted
	originalSecret := secret.DeepCopy()
	controllerutil.RemoveFinalizer(&secret, v1alpha1.AzureSecretFinalizer)
	err = s.client.Patch(ctx, &secret, client.MergeFrom(originalSecret))
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from secret %s for bucket %s: %w", bucket.Spec.Name, bucket.Spec.Name, err)
	}
	// We delete the secret
	err = s.client.Delete(ctx, &secret)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s for bucket %s: %w", bucket.Spec.Name, bucket.Spec.Name, err)
	}
	s.logger.Info(fmt.Sprintf("deleted secret %s", bucket.Spec.Name))

	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob) and tags on the Storage Container
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return s.setLifecycleRules(ctx, bucket)
}
