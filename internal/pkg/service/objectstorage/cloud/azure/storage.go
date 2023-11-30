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
	s.logger.Info("CHECK BUCKET EXISTENCE - Entering")
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
		bucket.Spec.Name,
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
	s.logger.Info("CREATION BUCKET - Entering")
	// Check if storage account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx)
	if err != nil {
		return err
	}
	// If StorageAccount does not exists, we need to create it first
	if !existsStorageAccount {
		// Create storage account
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
			return err
		}
		s.logger.Info(fmt.Sprintf("Storage Account %s created", s.storageAccountName))
	}

	return nil
}

// DeleteBucket deletes the Storage Container AND the Storage Account
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
