package azure

import (
	"context"
	"fmt"

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
		return nil, fmt.Errorf("missing management cluster AzureCluster CR for cluster %s in namespace %s: %w", c.ManagementCluster.Name, c.ManagementCluster.Namespace, err)
	}
	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		return nil, fmt.Errorf("failed to get identity name from AzureCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, fmt.Errorf("missing management cluster identityRef for cluster %s/%s", c.ManagementCluster.Namespace, c.ManagementCluster.Name)
	}
	clusterIdentityNamespace, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "namespace")
	if err != nil {
		return nil, fmt.Errorf("failed to get identity namespace from AzureCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	if !found || clusterIdentityNamespace == "" {
		logger.Info("Missing identity namespace, using management cluster namespace")
		clusterIdentityNamespace = c.ManagementCluster.Namespace
	}
	clusterIdentity, err := c.getClusterCRIdentity(ctx, clusterIdentityName, clusterIdentityNamespace)
	if err != nil {
		return nil, fmt.Errorf("missing management cluster identity AzureClusterIdentity CR %s/%s for cluster %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	clusterTags, found, err := unstructured.NestedStringMap(cluster.Object, "spec", "additionalTags")
	if err != nil {
		return nil, fmt.Errorf("failed to get additional tags from AzureCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	if !found || len(clusterTags) == 0 {
		logger.Info("No cluster tags found")
	}
	var secret = corev1.Secret{}
	resourceGroup, found, err := unstructured.NestedString(cluster.Object, "spec", "resourceGroup")
	if !found || err != nil {
		return nil, fmt.Errorf("missing or incorrect resourceGroup in AzureCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	subscriptionID, found, err := unstructured.NestedString(cluster.Object, "spec", "subscriptionID")
	if !found || err != nil {
		return nil, fmt.Errorf("missing or incorrect subscriptionID in AzureCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	typeIdentity, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "type")
	if !found || err != nil {
		return nil, fmt.Errorf("missing or incorrect identity type in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
	}
	clientID, tenantID := "", ""
	switch typeIdentity {
	case "UserAssignedMSI":
		clientID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientID")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect clientID in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
	case "ManualServicePrincipal":
		tenantID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "tenantID")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect tenantID in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
		clientID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientID")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect clientID in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
		clientSecretName, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "clientSecret", "name")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect clientSecret.name in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
		clientSecretNamespace, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "clientSecret", "namespace")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect clientSecret.namespace in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
		err = c.Client.Get(
			ctx,
			types.NamespacedName{
				Namespace: clientSecretNamespace,
				Name:      clientSecretName,
			},
			&secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get client secret %s/%s for AzureClusterIdentity %s/%s: %w", clientSecretNamespace, clientSecretName, clusterIdentityNamespace, clusterIdentityName, err)
		}
	case "WorkloadIdentity":
		tenantID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "tenantID")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect tenantID in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
		clientID, found, err = unstructured.NestedString(clusterIdentity.Object, "spec", "clientID")
		if !found || err != nil {
			return nil, fmt.Errorf("missing or incorrect clientID in AzureClusterIdentity %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
		}
	default:
		return nil, fmt.Errorf("unsupported identity type %s in AzureClusterIdentity %s/%s", typeIdentity, clusterIdentityNamespace, clusterIdentityName)
	}

	return AzureCluster{
		Client:     c.Client,
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
	if err != nil {
		return cluster, fmt.Errorf("failed to get AzureCluster CR %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	return cluster, nil
}

func (c AzureClusterGetter) getClusterCRIdentity(ctx context.Context, clusterIdentityName string, clusterIdentityNamespace string) (*unstructured.Unstructured, error) {
	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})

	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(clusterIdentityName, clusterIdentityNamespace), clusterIdentity)
	if err != nil {
		return clusterIdentity, fmt.Errorf("failed to get AzureClusterIdentity CR %s/%s: %w", clusterIdentityNamespace, clusterIdentityName, err)
	}
	return clusterIdentity, nil
}

// AzureCluster implements Cluster Interface with Azure data
type AzureCluster struct {
	Client      client.Client
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

func (c AzureCluster) GetResourceGroup() string {
	return c.Credentials.ResourceGroup
}

func (c AzureCluster) GetSubscriptionID() string {
	return c.Credentials.SubscriptionID
}

func (c AzureCluster) GetVNetName() string {
	return c.GetName() + "-vnet"
}
