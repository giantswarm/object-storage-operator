package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type AzureObjectStorageAdapter struct {
	storageAccountClient *armstorage.AccountsClient
	blobContainerClient  *armstorage.BlobContainersClient
	logger               logr.Logger
	cluster              AzureCluster
	storageAccountName   string
}

func NewAzureStorageService(storageAccountClient *armstorage.AccountsClient, blobContainerClient *armstorage.BlobContainersClient, logger logr.Logger, cluster AzureCluster) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		storageAccountClient: storageAccountClient,
		blobContainerClient:  blobContainerClient,
		logger:               logger,
		cluster:              cluster,
		// We choose to name storage account <installationName>observability (Azure requirements avoid special character like "-")
		storageAccountName: cluster.Name + "observability",
	}
}

// ExistsBucket checks if the bucket exists on Azure
// firstly, it checks if the Storage Account exists
// then, it checks if the Blob Container exists
func (s AzureObjectStorageAdapter) ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	// Check if storage account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx)
	if err != nil {
		return false, err
	}
	// If StorageAccount does not exists that means the bucket does not exists too, so we return false
	if !existsStorageAccount {
		return false, nil
	}
	// Check BlobContainer name exists in StorageAccount
	_, err = s.blobContainerClient.Get(
		ctx,
		s.cluster.Credentials.ResourceGroup,
		s.storageAccountName,
		bucket.Spec.Name,
		nil)
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

// ExistsStorageAccount checks Storage Account existence on Azure
func (s AzureObjectStorageAdapter) existsStorageAccount(ctx context.Context) (bool, error) {
	availability, err := s.storageAccountClient.CheckNameAvailability(
		ctx,
		armstorage.AccountCheckNameAvailabilityParameters{
			Name: to.Ptr(s.storageAccountName),
			Type: to.Ptr("Microsoft.Storage/storageAccounts"),
		},
		nil)
	if err != nil {
		return false, err
	}
	return !*availability.NameAvailable, nil
}

// CreateBucket creates the Storage Account if it not exists AND the Storage Container
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	// Check if Storage Account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx)
	if err != nil {
		return err
	}
	// If Storage Account does not exists, we need to create it first
	if !existsStorageAccount {
		// Create Storage Account
		pollerResp, err := s.storageAccountClient.BeginCreate(
			ctx,
			s.cluster.Credentials.ResourceGroup,
			s.storageAccountName,
			armstorage.AccountCreateParameters{
				Kind: to.Ptr(armstorage.KindBlobStorage),
				SKU: &armstorage.SKU{
					Name: to.Ptr(armstorage.SKUNameStandardLRS),
				},
				Location: to.Ptr(s.cluster.Region),
				Properties: &armstorage.AccountPropertiesCreateParameters{
					AccessTier: to.Ptr(armstorage.AccessTierCool),
					Encryption: &armstorage.Encryption{
						Services: &armstorage.EncryptionServices{
							Blob: &armstorage.EncryptionService{
								KeyType: to.Ptr(armstorage.KeyTypeAccount),
								Enabled: to.Ptr(true),
							},
						},
						KeySource: to.Ptr(armstorage.KeySourceMicrosoftStorage),
					},
				},
			}, nil)
		if err != nil {
			return err
		}
		_, err = pollerResp.PollUntilDone(ctx, nil)
		if err != nil {
			s.logger.Error(err, fmt.Sprintf("Error creating Storage Account %s", s.storageAccountName))
			return err
		}
		s.logger.Info(fmt.Sprintf("Storage Account %s created", s.storageAccountName))
	}

	// Create Storage Container
	_, err = s.blobContainerClient.Create(
		ctx,
		s.cluster.Credentials.ResourceGroup,
		s.storageAccountName,
		bucket.Spec.Name,
		armstorage.BlobContainer{
			ContainerProperties: &armstorage.ContainerProperties{
				PublicAccess: to.Ptr(armstorage.PublicAccessNone),
			},
		},
		nil,
	)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Error creating Storage Container %s", bucket.Spec.Name))
		return err
	}
	s.logger.Info(fmt.Sprintf("Storage Container %s created", bucket.Spec.Name))

	return nil
}

// DeleteBucket deletes the Storage Container (we don't delete the Storage Account because it may be useful for other observability resources)
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.blobContainerClient.Delete(
		ctx,
		s.cluster.Credentials.ResourceGroup,
		s.storageAccountName,
		bucket.Spec.Name,
		nil,
	)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Error deleting Storage Container %s", bucket.Spec.Name))
		return err
	}
	s.logger.Info(fmt.Sprintf("Storage Container %s deleted", bucket.Spec.Name))

	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob)
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	var err error
	// If expiration is not set, we remove all lifecycle rules
	err = s.setLifecycleRules(ctx, bucket)
	if err != nil {
		return err
	}

	err = s.setTags(ctx, bucket)
	return err
}

func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	//TODO
	return nil
}

// setTags set cluster additionalTags and bucket tags into Storage Container Metadata
func (s AzureObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	tags := map[string]*string{}
	for _, t := range bucket.Spec.Tags {
		// We use this to avoid pointer issues in range loops.
		tag := t
		if tag.Key != "" && tag.Value != "" {
			tags[tag.Key] = &tag.Value
		}
	}
	for k, v := range s.cluster.Tags {
		// We use this to avoid pointer issues in range loops.
		key := k
		value := v
		if key != "" && value != "" {
			tags[key] = &value
		}
	}

	// Updating Storage Container Metadata with tags (cluster additionalTags + Bucket tags)
	_, err := s.blobContainerClient.Update(
		ctx,
		s.cluster.Credentials.ResourceGroup,
		s.storageAccountName,
		bucket.Spec.Name,
		armstorage.BlobContainer{
			ContainerProperties: &armstorage.ContainerProperties{
				Metadata: tags,
			},
		},
		nil,
	)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Error updating Storage Container %s Metadata", bucket.Spec.Name))
	}
	return err
}
