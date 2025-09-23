package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage/v2"
	"github.com/aquilax/truncate"
	sanitize "github.com/mrz1836/go-sanitize"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

func (s AzureObjectStorageAdapter) upsertStorageAccount(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string, isPrivateManagementCluster bool) error {
	// Check if Storage Account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx, storageAccountName)
	if err != nil {
		return fmt.Errorf("failed to check if storage account %s exists: %w", storageAccountName, err)
	}

	publicNetworkAccess := armstorage.PublicNetworkAccessEnabled
	if isPrivateManagementCluster {
		publicNetworkAccess = armstorage.PublicNetworkAccessDisabled
	}

	// Create or Update Storage Account
	pollerStorageAccount, err := s.storageAccountClient.BeginCreate(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		armstorage.AccountCreateParameters{
			Kind: to.Ptr(armstorage.KindBlobStorage),
			SKU: &armstorage.SKU{
				Name: to.Ptr(armstorage.SKUNameStandardLRS),
			},
			Location: to.Ptr(s.cluster.GetRegion()),
			Properties: &armstorage.AccountPropertiesCreateParameters{
				AllowSharedKeyAccess: to.Ptr(true),
				AccessTier:           to.Ptr(armstorage.AccessTierHot),
				Encryption: &armstorage.Encryption{
					Services: &armstorage.EncryptionServices{
						Blob: &armstorage.EncryptionService{
							KeyType: to.Ptr(armstorage.KeyTypeAccount),
							Enabled: to.Ptr(true),
						},
					},
					KeySource: to.Ptr(armstorage.KeySourceMicrosoftStorage),
				},
				EnableHTTPSTrafficOnly: to.Ptr(true),
				MinimumTLSVersion:      to.Ptr(armstorage.MinimumTLSVersionTLS12),
				PublicNetworkAccess:    to.Ptr(publicNetworkAccess),
			},
			Tags: s.getBucketTags(bucket),
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin create storage account %s: %w", storageAccountName, err)
	}
	_, err = pollerStorageAccount.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete storage account %s creation: %w", storageAccountName, err)
	}

	if !existsStorageAccount {
		s.logger.Info(fmt.Sprintf("storage account %s created", storageAccountName))
	} else {
		s.logger.Info(fmt.Sprintf("storage account %s updated", storageAccountName))
	}
	return nil
}

func (s AzureObjectStorageAdapter) deleteStorageAccount(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string) error {
	// Delete Storage Account
	// We delete the Storage Account, which delete the Storage Container
	_, err := s.storageAccountClient.Delete(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to delete storage account %s for bucket %s: %w", storageAccountName, bucket.Spec.Name, err)
	}
	s.logger.Info(fmt.Sprintf("storage account %s and storage container %s deleted", storageAccountName, bucket.Spec.Name))
	return nil
}

// existsStorageAccount checks if a storage account exists for the given bucket name in Azure Object Storage.
// It returns a boolean indicating whether the storage account exists or not, along with any error encountered.
func (s AzureObjectStorageAdapter) existsStorageAccount(ctx context.Context, storageAccountName string) (bool, error) {
	availability, err := s.storageAccountClient.CheckNameAvailability(
		ctx,
		armstorage.AccountCheckNameAvailabilityParameters{
			Name: to.Ptr(storageAccountName),
			Type: to.Ptr("Microsoft.Storage/storageAccounts"),
		},
		nil)
	if err != nil {
		return false, fmt.Errorf("failed to check name availability for storage account %s: %w", storageAccountName, err)
	}
	return !*availability.NameAvailable, nil
}

// setLifecycleRules set a lifecycle rule on the Storage Account to delete Blobs older than X days
func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := sanitizeStorageAccountName(bucket.Spec.Name)
	if bucket.Spec.ExpirationPolicy != nil {
		_, err := s.managementPoliciesClient.CreateOrUpdate(
			ctx,
			s.cluster.GetResourceGroup(),
			storageAccountName,
			armstorage.ManagementPolicyNameDefault,
			armstorage.ManagementPolicy{
				Properties: &armstorage.ManagementPolicyProperties{
					Policy: &armstorage.ManagementPolicySchema{
						Rules: []*armstorage.ManagementPolicyRule{
							{
								Enabled: to.Ptr(true),
								Name:    to.Ptr(LifecycleRuleName),
								Type:    to.Ptr(armstorage.RuleTypeLifecycle),
								Definition: &armstorage.ManagementPolicyDefinition{
									Actions: &armstorage.ManagementPolicyAction{
										BaseBlob: &armstorage.ManagementPolicyBaseBlob{
											Delete: &armstorage.DateAfterModification{
												DaysAfterModificationGreaterThan: to.Ptr(float32(bucket.Spec.ExpirationPolicy.Days)),
											},
										},
									},
									Filters: &armstorage.ManagementPolicyFilter{
										BlobTypes: []*string{
											to.Ptr("blockBlob"),
										},
									},
								},
							},
						},
					},
				},
			},
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create/update lifecycle policy for storage account %s: %w", storageAccountName, err)
		}
		return nil
	}

	// No Lifecycle Policy defines in the bucket CR, we delete it in the Storage Account
	_, err := s.managementPoliciesClient.Delete(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		LifecycleRuleName,
		nil,
	)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			// If the Lifecycle policy does not exists, it's not an error
			if respErr.StatusCode == http.StatusNotFound {
				return nil
			}
		}
		return fmt.Errorf("failed to delete lifecycle policy for storage account %s: %w", storageAccountName, err)
	}
	return nil
}

// sanitizeStorageAccountName sanitizes the given name by removing any non-alphanumeric characters and truncating it to a maximum length of 24 characters.
// more details https://learn.microsoft.com/en-us/rest/api/storagerp/storage-accounts/get-properties?view=rest-storagerp-2023-01-01&tabs=HTTP#uri-parameters
func sanitizeStorageAccountName(name string) string {
	return truncate.Truncate(sanitize.AlphaNumeric(name, false), 24, "", truncate.PositionEnd)
}
