package azure

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/flags"
)

// AzureClusterGetter implements ClusterGetter Interface
// It creates an AzureCluster object
type AzureClusterGetter struct {
	Client            client.Client
	ManagementCluster flags.ManagementCluster
}

const (
	Group                  = "infrastructure.cluster.x-k8s.io"
	KindCluster            = "AzureCluster"
	VersionCluster         = "v1beta1"
	KindClusterIdentity    = "AzureClusterIdentity"
	VersionClusterIdentity = "v1beta1"
	ClientSecretKeyName    = "clientSecret"
)

func (c AzureClusterGetter) GetCluster(ctx context.Context) (cluster.Cluster, error) {
	logger := log.FromContext(ctx)

	cluster, err := c.getClusterCR(ctx)
	if err != nil {
		logger.Error(err, "Missing management cluster AzureCluster CR")
		return nil, errors.WithStack(err)
	}
	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return nil, errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, errors.New("missing management cluster identify")
	}
	clusterIdentity, err := c.getClusterCRIdentiy(ctx, clusterIdentityName)
	if err != nil {
		logger.Error(err, "Missing management cluster identity AzureClusterIdentity CR")
		return nil, errors.WithStack(err)
	}
	clusterTags, found, err := unstructured.NestedStringMap(cluster.Object, "spec", "additionalTags")
	if err != nil {
		logger.Error(err, "Additional tags are not a map")
		return nil, errors.WithStack(err)
	}
	if !found || len(clusterTags) == 0 {
		logger.Info("No cluster tags found")
	}
	var secret corev1.Secret
	resourceGroup, found, err := unstructured.NestedString(cluster.Object, "spec", "resourceGroup")
	if !found || err != nil {
		return nil, errors.New("Missing or incorrect resourceGroup")
	}
	subscriptionID, found, err := unstructured.NestedString(cluster.Object, "spec", "subscriptionID")
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
		err = c.Client.Get(ctx, clientSecretName, &secret)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return AzureCluster{
		Name:       c.ManagementCluster.Name,
		Namespace:  c.ManagementCluster.Namespace,
		BaseDomain: c.ManagementCluster.BaseDomain,
		Region:     c.ManagementCluster.Region,
		Tags:       clusterTags,
		Credentials: AzureCredentials{
			ResourceGroup:  resourceGroup,
			SubscriptionID: subscriptionID,
			TypeIdentity:   typeIdentity,
			ClientID:       clientID,
			TenantID:       tenantID,
			SecretRef:      secret,
		},
	}, nil
}

func (c AzureClusterGetter) getClusterCR(ctx context.Context) (*unstructured.Unstructured, error) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindCluster,
		Version: VersionCluster,
	})
	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(c.ManagementCluster.Name, c.ManagementCluster.Namespace), cluster)
	return cluster, errors.WithStack(err)
}

func (c AzureClusterGetter) getClusterCRIdentiy(ctx context.Context, clusterIdentityName string) (*unstructured.Unstructured, error) {
	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})

	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(clusterIdentityName, c.ManagementCluster.Namespace), clusterIdentity)
	return clusterIdentity, errors.WithStack(err)
}

// AzureCluster implements Cluster Interface with Azure data
type AzureCluster struct {
	Name        string
	Namespace   string
	BaseDomain  string
	Region      string
	Tags        map[string]string
	Credentials AzureCredentials
}

type AzureCredentials struct {
	ResourceGroup  string
	SubscriptionID string
	TypeIdentity   string
	ClientID       string
	TenantID       string
	SecretRef      corev1.Secret
}

func (c AzureCluster) GetName() string {
	return c.Name
}

func (c AzureCluster) GetNamespace() string {
	return c.Namespace
}

func (c AzureCluster) GetBaseDomain() string {
	return c.BaseDomain
}

func (c AzureCluster) GetRegion() string {
	return c.Region
}

func (c AzureCluster) GetTags() map[string]string {
	return c.Tags
}

func (c AzureCluster) GetCredentials() cluster.Credentials {
	return c.Credentials
}
