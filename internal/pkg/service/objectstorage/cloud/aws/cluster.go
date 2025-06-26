package aws

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/flags"
)

// AWSClusterGetter implements ClusterGetter Interface
// It creates an AWSCluster object
type AWSClusterGetter struct {
	Client            client.Client
	ManagementCluster flags.ManagementCluster
}

const (
	Group                  = "infrastructure.cluster.x-k8s.io"
	KindCluster            = "AWSCluster"
	VersionCluster         = "v1beta2"
	KindClusterIdentity    = "AWSClusterRoleIdentity"
	VersionClusterIdentity = "v1beta2"
)

func (c AWSClusterGetter) GetCluster(ctx context.Context) (cluster.Cluster, error) {
	logger := log.FromContext(ctx)

	cluster, err := c.getClusterCR(ctx)
	if err != nil {
		return nil, fmt.Errorf("missing management cluster AWSCluster CR for cluster %s in namespace %s: %w", c.ManagementCluster.Name, c.ManagementCluster.Namespace, err)
	}

	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		return nil, fmt.Errorf("failed to get identity name from AWSCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, fmt.Errorf("missing management cluster identityRef for cluster %s/%s", c.ManagementCluster.Namespace, c.ManagementCluster.Name)
	}
	clusterIdentity, err := c.getClusterCRIdentiy(ctx, clusterIdentityName)
	if err != nil {
		return nil, fmt.Errorf("missing management cluster identity AWSClusterRoleIdentity CR %s for cluster %s/%s: %w", clusterIdentityName, c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}

	roleArn, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "roleARN")
	if err != nil {
		return nil, fmt.Errorf("failed to get role ARN from AWSClusterRoleIdentity %s: %w", clusterIdentityName, err)
	}
	if !found {
		return nil, fmt.Errorf("missing role ARN in AWSClusterRoleIdentity %s for cluster %s/%s", clusterIdentityName, c.ManagementCluster.Namespace, c.ManagementCluster.Name)
	}

	clusterTags, found, err := unstructured.NestedStringMap(cluster.Object, "spec", "additionalTags")
	if err != nil {
		return nil, fmt.Errorf("failed to get additional tags from AWSCluster %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	if !found || len(clusterTags) == 0 {
		logger.Info("No cluster tags found")
	}

	return AWSCluster{
		Client:     c.Client,
		Name:       c.ManagementCluster.Name,
		Namespace:  c.ManagementCluster.Namespace,
		BaseDomain: c.ManagementCluster.BaseDomain,
		Region:     c.ManagementCluster.Region,
		Tags:       clusterTags,
		Credentials: AWSCredentials{
			Role: roleArn,
		},
	}, nil
}

func (c AWSClusterGetter) getClusterCR(ctx context.Context) (*unstructured.Unstructured, error) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindCluster,
		Version: VersionCluster,
	})
	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(c.ManagementCluster.Name, c.ManagementCluster.Namespace), cluster)
	if err != nil {
		return cluster, fmt.Errorf("failed to get AWSCluster CR %s/%s: %w", c.ManagementCluster.Namespace, c.ManagementCluster.Name, err)
	}
	return cluster, nil
}

func (c AWSClusterGetter) getClusterCRIdentiy(ctx context.Context, clusterIdentityName string) (*unstructured.Unstructured, error) {
	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})

	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(clusterIdentityName, c.ManagementCluster.Namespace), clusterIdentity)
	if err != nil {
		return clusterIdentity, fmt.Errorf("failed to get AWSClusterRoleIdentity CR %s/%s: %w", c.ManagementCluster.Namespace, clusterIdentityName, err)
	}
	return clusterIdentity, nil
}

// AWSCluster implements Cluster Interface with AWS data
type AWSCluster struct {
	Client      client.Client
	Name        string
	Namespace   string
	BaseDomain  string
	Region      string
	Tags        map[string]string
	Credentials AWSCredentials
}

type AWSCredentials struct {
	Role string
}

func (c AWSCluster) GetName() string {
	return c.Name
}

func (c AWSCluster) GetNamespace() string {
	return c.Namespace
}

func (c AWSCluster) GetBaseDomain() string {
	return c.BaseDomain
}

func (c AWSCluster) GetRegion() string {
	return c.Region
}

func (c AWSCluster) GetTags() map[string]string {
	return c.Tags
}

func (c AWSCluster) GetCredentials() cluster.Credentials {
	return c.Credentials
}
