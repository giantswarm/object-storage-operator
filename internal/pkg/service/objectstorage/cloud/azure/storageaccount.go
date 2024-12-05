package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/aquilax/truncate"
	sanitize "github.com/mrz1836/go-sanitize"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

func (s AzureObjectStorageAdapter) upsertStorageAccount(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string) error {
	// Check if Storage Account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx, storageAccountName)
	if err != nil {
		return err
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
				// TODO make sure this is not the case on gaggle or public installations
				PublicNetworkAccess: to.Ptr(armstorage.PublicNetworkAccessDisabled),
			},
			Tags: s.getBucketTags(bucket),
		}, nil)
	if err != nil {
		return err
	}
	_, err = pollerStorageAccount.PollUntilDone(ctx, nil)
	if err != nil {
		return err
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
		s.logger.Error(err, fmt.Sprintf("error deleting storage account %s and storage container %s", storageAccountName, bucket.Spec.Name))
		return err
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
		return false, err
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
												DaysAfterModificationGreaterThan: to.Ptr[float32](float32(bucket.Spec.ExpirationPolicy.Days)),
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
			s.logger.Error(err, fmt.Sprintf("Error creating/updating Policy Rule for Storage Account %s", storageAccountName))
		}
		return err
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
	}
	return err
}

// sanitizeStorageAccountName sanitizes the given name by removing any non-alphanumeric characters and truncating it to a maximum length of 24 characters.
// more details https://learn.microsoft.com/en-us/rest/api/storagerp/storage-accounts/get-properties?view=rest-storagerp-2023-01-01&tabs=HTTP#uri-parameters
func sanitizeStorageAccountName(name string) string {
	return truncate.Truncate(sanitize.AlphaNumeric(name, false), 24, "", truncate.PositionEnd)
}
