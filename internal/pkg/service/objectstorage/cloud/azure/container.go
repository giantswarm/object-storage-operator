package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage/v2"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

func (s AzureObjectStorageAdapter) existsContainer(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string) (bool, error) {
	// Check BlobContainer name exists in StorageAccount
	_, err := s.blobContainerClient.Get(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		bucket.Spec.Name,
		nil,
	)

	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			// If NOT FOUND error, that means the BlobContainer doesn't exist, so we return false
			if respErr.StatusCode == http.StatusNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func (s AzureObjectStorageAdapter) upsertContainer(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string) error {
	existsContainer, err := s.existsContainer(ctx, bucket, storageAccountName)
	if err != nil {
		return fmt.Errorf("failed to check if container %s exists in storage account %s: %w", bucket.Spec.Name, storageAccountName, err)
	}
	if !existsContainer {
		// Create Storage Container
		_, err := s.blobContainerClient.Create(
			ctx,
			s.cluster.GetResourceGroup(),
			storageAccountName,
			bucket.Spec.Name,
			armstorage.BlobContainer{
				ContainerProperties: &armstorage.ContainerProperties{
					PublicAccess: to.Ptr(armstorage.PublicAccessNone),
					Metadata:     s.getBucketTags(bucket),
				},
			},
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create storage container %s in storage account %s: %w", bucket.Spec.Name, storageAccountName, err)
		}
		s.logger.Info(fmt.Sprintf("storage container %s created", bucket.Spec.Name))
	} else {
		_, err := s.blobContainerClient.Update(
			ctx,
			s.cluster.GetResourceGroup(),
			storageAccountName,
			bucket.Spec.Name,
			armstorage.BlobContainer{
				ContainerProperties: &armstorage.ContainerProperties{
					Metadata: s.getBucketTags(bucket),
				},
			},
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to update storage container %s in storage account %s: %w", bucket.Spec.Name, storageAccountName, err)
		}
		s.logger.Info(fmt.Sprintf("storage container %s updated", bucket.Spec.Name))
	}

	return nil
}
