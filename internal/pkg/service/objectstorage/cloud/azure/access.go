package azure

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/pkg/errors"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

const (
	scope = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s/blobServices/default/containers/%s"
	// Role id for `Storage Blob Data Contributor`
	storageBlobDataContributorRoleID = "ba92f5b4-2d11-453d-a403-e96b0029c9fe"
)

type AzureAccessServiceAdapter struct {
	graphClient           *msgraphsdk.GraphServiceClient
	roleAssignmentsClient *armauthorization.RoleAssignmentsClient
	logger                logr.Logger
	cluster               AzureCluster
}

func NewAzureAccessService(logger logr.Logger, cluster AzureCluster) (*AzureAccessServiceAdapter, error) {
	azureCredentials, ok := cluster.GetCredentials().(AzureCredentials)
	if !ok {
		return nil, errors.New("could not cast cluster credentials into azure cluster credentials")
	}

	credentials, err := getAzureTokenCredentials(azureCredentials)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	armClientFactory, err := armauthorization.NewClientFactory(azureCredentials.SubscriptionID, credentials, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(credentials, []string{"User.Read"})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &AzureAccessServiceAdapter{
		graphClient:           graphClient,
		roleAssignmentsClient: armClientFactory.NewRoleAssignmentsClient(),
		logger:                logger,
		cluster:               cluster,
	}, nil
}

func (s AzureAccessServiceAdapter) templateScope(bucket *v1alpha1.Bucket) string {
	return fmt.Sprintf(scope, s.cluster.GetSubscriptionID(), s.cluster.GetResourceGroup(), getStorageAccountName(bucket.Spec.Name), bucket.Spec.Name)
}

func (s AzureAccessServiceAdapter) ConfigureRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	servicePrincipals, err := s.graphClient.ServicePrincipalsWithAppId(to.Ptr(s.cluster.GetCredentials().(AzureCredentials).ClientID)).Get(context.Background(), nil)
	if err != nil {
		return errors.WithStack(err)
	}

	s.logger.Info("Service principal", "servicePrincipals", servicePrincipals)

	if bucket.Status.BucketAzureRoleAssignmentID == "" {
		bucket.Status.BucketAzureRoleAssignmentID = uuid.New().String()
	}

	scope := fmt.Sprintf(scope, s.cluster.GetSubscriptionID(), s.cluster.GetResourceGroup(), getStorageAccountName(bucket.Spec.Name), bucket.Spec.Name)
	_, err = s.roleAssignmentsClient.Create(ctx, scope, bucket.Status.BucketAzureRoleAssignmentID, armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      to.Ptr(s.cluster.GetCredentials().(AzureCredentials).ClientID),
			PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
			RoleDefinitionID: to.Ptr(fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", s.cluster.GetSubscriptionID(), storageBlobDataContributorRoleID))},
	}, nil)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s AzureAccessServiceAdapter) DeleteRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	if bucket.Status.BucketAzureRoleAssignmentID == "" {
		return nil
	}

	_, err := s.roleAssignmentsClient.Get(ctx, s.templateScope(bucket), bucket.Spec.Name, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			// If NOT FOUND error, that means the role assigment doesn't exist, so we ignore it
			if respErr.StatusCode == http.StatusNotFound {
				return nil
			}
		}
		return errors.WithStack(err)
	}

	// Delete role assignment for the Storage Account so this operator can list blobs
	_, err = s.roleAssignmentsClient.Delete(ctx, s.templateScope(bucket), bucket.Status.BucketAzureRoleAssignmentID, nil)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
