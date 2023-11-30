package aws

import (
	"context"

	"github.com/pkg/errors"
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
		logger.Error(err, "Missing management cluster AWSCluster CR")
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
		logger.Error(err, "Missing management cluster identity AWSClusterRoleIdentity CR")
		return nil, errors.WithStack(err)
	}

	roleArn, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "roleARN")
	if err != nil {
		logger.Error(err, "Role arn is not a string")
		return nil, errors.WithStack(err)
	}
	if !found {
		return nil, errors.New("missing role arn")
	}

	clusterTags, found, err := unstructured.NestedStringMap(cluster.Object, "spec", "additionalTags")
	if err != nil {
		logger.Error(err, "Additional tags are not a map")
		return nil, errors.WithStack(err)
	}
	if !found || len(clusterTags) == 0 {
		logger.Info("No cluster tags found")
	}

	return AWSCluster{
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
	return cluster, errors.WithStack(err)
}

func (c AWSClusterGetter) getClusterCRIdentiy(ctx context.Context, clusterIdentityName string) (*unstructured.Unstructured, error) {
	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})

	err := c.Client.Get(ctx, c.ManagementCluster.ToObjectKey(clusterIdentityName, c.ManagementCluster.Namespace), clusterIdentity)
	return clusterIdentity, errors.WithStack(err)
}

// AWSCluster implements Cluster Interface with AWS data
type AWSCluster struct {
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
