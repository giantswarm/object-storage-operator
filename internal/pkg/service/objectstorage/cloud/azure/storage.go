package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

// ExistsBucket checks if the bucket exists on Azure
// firstly, it checks if the Storage Account exists
// then, it checks if the Blob Container exists
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

// ExistsStorageAccount checks Storage Account existence on Azure
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
						v1alpha1.BucketFinalizer,
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
	controllerutil.RemoveFinalizer(&secret, v1alpha1.BucketFinalizer)
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

func sanitizeTagKey(tagName string) string {
	return strings.ReplaceAll(tagName, "-", "_")
}

// setTags set cluster additionalTags and bucket tags into Storage Container Metadata
func (s AzureObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	storageAccountName := s.getStorageAccountName(bucket.Spec.Name)
	tags := map[string]*string{}
	for _, t := range bucket.Spec.Tags {
		// We use this to avoid pointer issues in range loops.
		tag := t
		if tag.Key != "" && tag.Value != "" {
			tags[sanitizeTagKey(tag.Key)] = &tag.Value
		}
	}
	for k, v := range s.cluster.GetTags() {
		// We use this to avoid pointer issues in range loops.
		key := k
		value := v
		if key != "" && value != "" {
			tags[sanitizeTagKey(key)] = &value
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

// getStorageAccountName returns the sanitized bucket name if already computed or compute it and return it
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

// sanitizeAlphanumeric24 returns the name following Azure rules (alphanumerical characters only + 24 characters MAX)
// more details https://learn.microsoft.com/en-us/rest/api/storagerp/storage-accounts/get-properties?view=rest-storagerp-2023-01-01&tabs=HTTP#uri-parameters
func sanitizeAlphanumeric24(name string) string {
	return truncate.Truncate(sanitize.AlphaNumeric(name, false), 24, "", truncate.PositionEnd)
}
