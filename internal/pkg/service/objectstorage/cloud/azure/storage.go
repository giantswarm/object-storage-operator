package azure

import (
	"context"
	"errors"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type AzureObjectStorageAdapter struct {
	azStorageAccountClient *armstorage.AccountsClient
	azBlobContainerClient  *armstorage.BlobContainersClient
	logger                 logr.Logger
	resourceGroup          string
}

func NewAzureStorageService(azStorageAccountClient *armstorage.AccountsClient, azBlobContainerClient *armstorage.BlobContainersClient, logger logr.Logger, resourceGroup string) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		azStorageAccountClient: azStorageAccountClient,
		azBlobContainerClient:  azBlobContainerClient,
		logger:                 logger,
		resourceGroup:          resourceGroup,
	}
}

// ExistsBucket checks if the bucket exists on Azure
// firstly, it checks if the Storage Account exists
// then, it checks if the Blob Container exists
func (s AzureObjectStorageAdapter) ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	// Check StorageAccount on Azure
	availability, err := s.azStorageAccountClient.CheckNameAvailability(
		ctx,
		armstorage.AccountCheckNameAvailabilityParameters{
			Name: to.Ptr(bucket.Spec.Name),
			Type: to.Ptr("Microsoft.Storage/storageAccounts"),
		},
		nil)
	if err != nil {
		return false, err
	}
	// If StorageAccount name is available that means it doesn't exist, so we return false
	if *availability.NameAvailable {
		return false, nil
	}
	// Check BlobContainer name exists in StorageAccount
	_, err = s.azBlobContainerClient.Get(
		ctx,
		s.resourceGroup,
		bucket.Spec.Name,
		bucket.Spec.Name,
		nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			// If NOT FOUND error, that means the BlobContainer doesn't exist, so we return false
			if respErr.StatusCode == http.StatusNotFound {
				return false, nil
			} else {
				return false, err
			}
		}
	}

	return true, nil
}

// CreateBucket create the Storage Account if it not exists AND the Storage Container
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	s.logger.Info("CREATION BUCKET - Entering")
	return nil
}

// DeleteBucket delete the Storage Container AND the Storage Account
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	s.logger.Info("DELETION BUCKET - Entering")
	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob)
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket, additionalTags map[string]string) error {
	var err error
	// If expiration is not set, we remove all lifecycle rules
	err = s.setLifecycleRules(ctx, bucket)
	if err != nil {
		return err
	}

	err = s.setTags(ctx, bucket, additionalTags)
	return err
}

func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

func (s AzureObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket, additionalTags map[string]string) error {
	//TODO
	return nil
}
