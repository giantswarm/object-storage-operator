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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	storageAccountName       string
}

func NewAzureStorageService(storageAccountClient *armstorage.AccountsClient, blobContainerClient *armstorage.BlobContainersClient, managementPoliciesClient *armstorage.ManagementPoliciesClient, logger logr.Logger, cluster AzureCluster) AzureObjectStorageAdapter {
	return AzureObjectStorageAdapter{
		storageAccountClient:     storageAccountClient,
		blobContainerClient:      blobContainerClient,
		managementPoliciesClient: managementPoliciesClient,
		logger:                   logger,
		cluster:                  cluster,
		// We choose to name storage account <installationName>observability (Azure requirements avoid special character like "-")
		storageAccountName: cluster.GetName() + "observability",
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
		s.cluster.GetResourceGroup(),
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
			s.cluster.GetResourceGroup(),
			s.storageAccountName,
			armstorage.AccountCreateParameters{
				Kind: to.Ptr(armstorage.KindBlobStorage),
				SKU: &armstorage.SKU{
					Name: to.Ptr(armstorage.SKUNameStandardLRS),
				},
				Location: to.Ptr(s.cluster.GetRegion()),
				Properties: &armstorage.AccountPropertiesCreateParameters{
					AccessTier: to.Ptr(armstorage.AccessTierHot),
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

	// Create Storage Container
	_, err = s.blobContainerClient.Create(
		ctx,
		s.cluster.GetResourceGroup(),
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
		return err
	}
	s.logger.Info(fmt.Sprintf("Storage Container %s created", bucket.Spec.Name))

	// Create a K8S Secret to store Storage Account Access Key
	// First, we retrieve Storage Account Access Key on Azure
	listKeys, err := s.storageAccountClient.ListKeys(
		ctx,
		s.cluster.GetResourceGroup(),
		s.storageAccountName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Impossible to retrieve Access Keys from Storage Account %s", s.storageAccountName)
	}
	// Then, we retrieve the Access Key for 'key1'
	secretName := s.cluster.GetName() + "-logging-secret"
	foundKey1 := false
	for _, k := range listKeys.Keys {
		if *k.KeyName == "key1" {
			foundKey1 = true
			// Finally, we create the Secret
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "loki",
				},
				Data: map[string][]byte{
					"key": []byte(*k.Value),
				},
			}
			err := s.cluster.GetClient().Create(ctx, secret, nil)
			if err != nil {
				return err
			}
			s.logger.Info(fmt.Sprintf("Secret %s created", secretName))
			break
		}
	}
	if !foundKey1 {
		return fmt.Errorf("Impossible to retrieve Access Keys 'key1' from Storage Account %s", s.storageAccountName)
	}

	return nil
}

// DeleteBucket deletes the Storage Account (Storage Container will be deleted by cascade)
// Here, we decided to have a Storage Account dedicated to a Storage Container (relation 1 - 1)
// We want to prevent the Storage Account from being used by anyone
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.storageAccountClient.Delete(
		ctx,
		s.cluster.GetResourceGroup(),
		s.storageAccountName,
		nil,
	)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Error deleting Storage Account %s and Storage Container %s", s.storageAccountName, bucket.Spec.Name))
		return err
	}
	s.logger.Info(fmt.Sprintf("Storage Account %s and Storage Container %s deleted", s.storageAccountName, bucket.Spec.Name))

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

// setLifecycleRules set a lifecycle rule on the Storage Account to delete Blobs older than X days
func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	if bucket.Spec.ExpirationPolicy != nil {
		_, err := s.managementPoliciesClient.CreateOrUpdate(
			ctx,
			s.cluster.GetResourceGroup(),
			s.storageAccountName,
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
			s.logger.Error(err, fmt.Sprintf("Error creating/updating Policy Rule for Storage Account %s", s.storageAccountName))
		}
		return err
	}

	// No Lifecycle Policy defines in the bucket CR, we delete it in the Storage Account
	_, err := s.managementPoliciesClient.Delete(
		ctx,
		s.cluster.GetResourceGroup(),
		s.storageAccountName,
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
	for k, v := range s.cluster.GetTags() {
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
		s.cluster.GetResourceGroup(),
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
