package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
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
	storageAccountClient     *armstorage.AccountsClient
	blobContainerClient      *armstorage.BlobContainersClient
	managementPoliciesClient *armstorage.ManagementPoliciesClient
	logger                   logr.Logger
	cluster                  AzureCluster
	client                   client.Client
	listStorageAccountName   []string
}

// NewAzureStorageService creates a new instance of AzureObjectStorageAdapter.
// It takes in the necessary parameters to initialize the adapter and returns the created instance.
// The storageAccountClient, blobContainerClient, and managementPoliciesClient are clients for interacting with Azure storage resources.
// The logger is used for logging purposes.
// The cluster represents the Azure cluster.
// The client is the Kubernetes client used for interacting with the Kubernetes API.
// The listStorageAccountName is a list of storage account names.
func NewAzureStorageService(storageAccountClient *armstorage.AccountsClient, blobContainerClient *armstorage.BlobContainersClient, managementPoliciesClient *armstorage.ManagementPoliciesClient, logger logr.Logger, cluster AzureCluster, client client.Client) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		storageAccountClient:     storageAccountClient,
		blobContainerClient:      blobContainerClient,
		managementPoliciesClient: managementPoliciesClient,
		logger:                   logger,
		cluster:                  cluster,
		client:                   client,
		listStorageAccountName:   []string{},
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

// CreateBucket creates the Storage Account if it not exists AND the Storage Container
// CreateBucket creates a bucket in Azure Object Storage.
// It checks if the storage account exists, and if not, it creates it.
// Then, it creates a storage container within the storage account.
// Finally, it retrieves the access key for 'key1' and creates a K8S Secret to store the storage account access key.
// The Secret is created in the same namespace as the bucket.
// The function returns an error if any of the operations fail.
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)

	if err := s.upsertStorageAccount(ctx, bucket, storageAccountName); err != nil {
		return err
	}

	if err := s.upsertContainer(ctx, bucket, storageAccountName); err != nil {
		return err
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
	// Then, we retrieve the Access Key for 'key1'
	foundKey1 := false
	for _, k := range listKeys.Keys {
		if *k.KeyName == "key1" {
			foundKey1 = true
			// Finally, we create the Secret into the bucket namespace
			secret := &v1.Secret{
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
				Data: map[string][]byte{
					"accountName": []byte(storageAccountName),
					"accountKey":  []byte(*k.Value),
				},
			}
			err := s.client.Create(ctx, secret)
			if err != nil {
				return err
			}
			s.logger.Info(fmt.Sprintf("created secret %s", bucket.Spec.Name))
			break
		}
	}
	if !foundKey1 {
		return fmt.Errorf("unable to retrieve access keys 'key1' from storage account %s", storageAccountName)
	}

	return nil
}

// UpdateBucket creates or updates the Storage Account AND the Storage Container if it does not exists
func (s AzureObjectStorageAdapter) UpdateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return s.CreateBucket(ctx, bucket)
}

// DeleteBucket deletes the Storage Account (Storage Container will be deleted by cascade)
// Here, we decided to have a Storage Account dedicated to a Storage Container (relation 1 - 1)
// We want to prevent the Storage Account from being used by anyone
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)

	if err := s.deleteStorageAccount(ctx, bucket, storageAccountName); err != nil {
		return err
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
		s.logger.Error(err, fmt.Sprintf("unable to retrieve secret %s", bucket.Spec.Name))
		return err
	}
	// We remove the finalizer to allow the secret to be deleted
	originalSecret := secret.DeepCopy()
	controllerutil.RemoveFinalizer(&secret, v1alpha1.AzureSecretFinalizer)
	err = s.client.Patch(ctx, &secret, client.MergeFrom(originalSecret))
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("unable to remove the finalizer in the secret %s", bucket.Spec.Name))
		return err
	}
	// We delete the secret
	err = s.client.Delete(ctx, &secret)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("unable to delete secret %s", bucket.Spec.Name))
		return err
	}
	s.logger.Info(fmt.Sprintf("deleted secret %s", bucket.Spec.Name))

	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob) and tags on the Storage Container
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return s.setLifecycleRules(ctx, bucket)
}
