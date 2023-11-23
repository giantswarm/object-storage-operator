package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

type AzureObjectStorageService struct {
}

func (s AzureObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	//TODO
	return nil, nil
}

func (s AzureObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, cli client.Client) (objectstorage.ObjectStorageService, error) {
	clust := &unstructured.Unstructured{}
	clust.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindCluster,
		Version: VersionCluster,
	})
	err := cli.Get(ctx, client.ObjectKey{
		Name:      cluster.GetName(),
		Namespace: cluster.GetNamespace(),
	}, clust)
	if err != nil {
		logger.Error(err, "Missing management cluster AzureCluster CR")
		return nil, errors.WithStack(err)
	}

	clusterIdentityName, found, err := unstructured.NestedString(clust.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return nil, errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, errors.New("missing management cluster identify")
	}

	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})
	err = cli.Get(ctx, client.ObjectKey{
		Name:      clusterIdentityName,
		Namespace: cluster.GetNamespace(),
	}, clusterIdentity)
	if err != nil {
		logger.Error(err, "Missing management cluster identity AzureClusterRoleIdentity CR")
		return nil, errors.WithStack(err)
	}

	clusterIdentityName, found, err = unstructured.NestedString(clust.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return nil, errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, errors.New("missing management cluster identify")
	}

	var cred azcore.TokenCredential
	subscriptionID, found, err := unstructured.NestedString(clust.Object, "spec", "subscriptionID")
	if !found || err != nil {
		return nil, errors.New("Missing or incorrect subscriptionID")
	}
	typeIdentity, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "type")
	if !found || err != nil {
		return nil, errors.New("Missing or incorrect identity.type")
	}
	clientID, tenantID, clientSecretName, clientSecretNamespace := "", "", "", ""
	if typeIdentity == "UserAssignedMSI" {
		clientID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientID")
		if !found || err != nil {
			return nil, errors.New("Missing or incorrect identity.clientID")
		}
		cred, err = azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(clientID),
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	if typeIdentity == "ManualServicePrincipal" {
		tenantID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "tenantID")
		if !found || err != nil {
			return nil, errors.New("Missing or incorrect identity.tenantID")
		}
		clientID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientID")
		if !found || err != nil {
			return nil, errors.New("Missing or incorrect identity.clientID")
		}
		clientSecretName, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientSecret", "name")
		if !found || err != nil {
			return nil, errors.New("Missing or incorrect identity.clientSecret.name")
		}
		clientSecretNamespace, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientSecret", "namespace")
		if !found || err != nil {
			return nil, errors.New("Missing or incorrect identity.clientSecret.namespace")
		}
		clientSecretName := types.NamespacedName{
			Namespace: clientSecretNamespace,
			Name:      clientSecretName,
		}
		secret := &corev1.Secret{}
		err = cli.Get(ctx, clientSecretName, secret)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cred, err = azidentity.NewClientSecretCredential(
			tenantID,
			clientID,
			string(secret.Data[ClientSecretKeyName]),
			nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	var storageClientFactory *armstorage.ClientFactory
	storageClientFactory, err = armstorage.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return NewAzureStorageService(storageClientFactory, logger), nil
}
