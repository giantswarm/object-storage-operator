package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type AzureObjectStorageAdapter struct {
	azClientFactory *armstorage.ClientFactory
	logger          logr.Logger
}

func NewAzureStorageService(azClientFactory *armstorage.ClientFactory, logger logr.Logger) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		azClientFactory: azClientFactory,
		logger:          logger,
	}
}

// ExistsBucket checks if the bucket exists on Azure
// firstly, it checks if the Storage Account exists
// then, it checks if the Storage Container exists
func (s AzureObjectStorageAdapter) ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	//TODO
	return true, nil
}

// CreateBucket create the Storage Account if it not exists AND the Storage Container
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

// DeleteBucket delete the Storage Container AND the Storage Account
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob)
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

func (s AzureObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}
