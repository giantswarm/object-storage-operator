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
	"github.com/go-logr/logr"
	sanitize "github.com/mrz1836/go-sanitize"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	// Check if storage account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx, bucket.Spec.Name)
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
		s.getStorageAccountName(bucket.Spec.Name),
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

// existsStorageAccount checks if a storage account exists for the given bucket name in Azure Object Storage.
// It returns a boolean indicating whether the storage account exists or not, along with any error encountered.
func (s AzureObjectStorageAdapter) existsStorageAccount(ctx context.Context, bucketName string) (bool, error) {
	availability, err := s.storageAccountClient.CheckNameAvailability(
		ctx,
		armstorage.AccountCheckNameAvailabilityParameters{
			Name: to.Ptr(s.getStorageAccountName(bucketName)),
			Type: to.Ptr("Microsoft.Storage/storageAccounts"),
		},
		nil)
	if err != nil {
		return false, err
	}
	return !*availability.NameAvailable, nil
}

// CreateBucket creates the Storage Account if it not exists AND the Storage Container
// CreateBucket creates a bucket in Azure Object Storage.
// It checks if the storage account exists, and if not, it creates it.
// Then, it creates a storage container within the storage account.
// Finally, it retrieves the access key for 'key1' and creates a K8S Secret to store the storage account access key.
// The Secret is created in the same namespace as the bucket.
// The function returns an error if any of the operations fail.
func (s AzureObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := s.getStorageAccountName(bucket.Spec.Name)
	// Check if Storage Account exists on Azure
	existsStorageAccount, err := s.existsStorageAccount(ctx, storageAccountName)
	if err != nil {
		return err
	}
	// If Storage Account does not exists, we need to create it first
	if !existsStorageAccount {
		// Create Storage Account
		pollerResp, err := s.storageAccountClient.BeginCreate(
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
		s.logger.Info(fmt.Sprintf("Storage Account %s created", storageAccountName))
	}

	// Create Storage Container
	_, err = s.blobContainerClient.Create(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
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
		storageAccountName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Impossible to retrieve Access Keys from Storage Account %s", storageAccountName)
	}
	// Then, we retrieve the Access Key for 'key1'
	foundKey1 := false
	for _, k := range listKeys.Keys {
		if *k.KeyName == "key1" {
			foundKey1 = true
			// Finally, we create the Secret into the bucket namespace
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      storageAccountName,
					Namespace: bucket.Namespace,
					Labels: map[string]string{
						"giantswarm.io/managed-by": "object-storage-operator",
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
			s.logger.Info(fmt.Sprintf("Secret %s created", storageAccountName))
			break
		}
	}
	if !foundKey1 {
		return fmt.Errorf("Impossible to retrieve Access Keys 'key1' from Storage Account %s", storageAccountName)
	}

	return nil
}

// DeleteBucket deletes the Storage Account (Storage Container will be deleted by cascade)
// Here, we decided to have a Storage Account dedicated to a Storage Container (relation 1 - 1)
// We want to prevent the Storage Account from being used by anyone
func (s AzureObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := s.getStorageAccountName(bucket.Spec.Name)
	// We delete the Storage Account, which delete the Storage Container
	_, err := s.storageAccountClient.Delete(
		ctx,
		s.cluster.GetResourceGroup(),
		storageAccountName,
		nil,
	)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Error deleting Storage Account %s and Storage Container %s", storageAccountName, bucket.Spec.Name))
		return err
	}
	s.logger.Info(fmt.Sprintf("Storage Account %s and Storage Container %s deleted", storageAccountName, bucket.Spec.Name))

	// We delete the Azure Credentials secret
	var secret = v1.Secret{}
	err = s.client.Get(
		ctx,
		types.NamespacedName{
			Namespace: bucket.Namespace,
			Name:      storageAccountName,
		},
		&secret)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Impossible to retrieve Secret %s", storageAccountName))
		return err
	}
	err = s.client.Delete(ctx, &secret)
	if err != nil {
		s.logger.Error(err, fmt.Sprintf("Impossible to delete Secret %s", storageAccountName))
		return err
	}
	s.logger.Info(fmt.Sprintf("Secret %s deleted", storageAccountName))

	return nil
}

// ConfigureBucket set lifecycle rules (expiration on blob) and tags on the Storage Container
func (s AzureObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	var err error
	err = s.setLifecycleRules(ctx, bucket)
	if err != nil {
		return err
	}

	err = s.setTags(ctx, bucket)
	return err
}

// setLifecycleRules set a lifecycle rule on the Storage Account to delete Blobs older than X days
func (s AzureObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := s.getStorageAccountName(bucket.Spec.Name)
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

// setTags set cluster additionalTags and bucket tags into Storage Container Metadata
func (s AzureObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := s.getStorageAccountName(bucket.Spec.Name)
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
		storageAccountName,
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

// getStorageAccountName returns the storage account name for the given bucket name.
// It sanitizes the bucket name and checks if it already exists in the list of storage account names.
// If it exists, it returns the sanitized name. Otherwise, it adds the sanitized name to the list and returns it.
func (s *AzureObjectStorageAdapter) getStorageAccountName(bucketName string) string {
	sanitizeName := sanitizeAlphanumeric24(bucketName)
	for _, name := range s.listStorageAccountName {
		if sanitizeName == name {
			return sanitizeName
		}
	}
	s.listStorageAccountName = append(s.listStorageAccountName, sanitizeName)
	return sanitizeName
}

// sanitizeAlphanumeric24 sanitizes the given name by removing any non-alphanumeric characters and truncating it to a maximum length of 24 characters.
// more details https://learn.microsoft.com/en-us/rest/api/storagerp/storage-accounts/get-properties?view=rest-storagerp-2023-01-01&tabs=HTTP#uri-parameters
func sanitizeAlphanumeric24(name string) string {
	return truncate.Truncate(sanitize.AlphaNumeric(name, false), 24, "", truncate.PositionEnd)
}
