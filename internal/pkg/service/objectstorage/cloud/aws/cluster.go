package aws

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
)

// AWSClusterGetter implements ClusterGetter Interface
// It creates an AWSCluster object
type AWSClusterGetter struct {
}

const (
	Group                  = "infrastructure.cluster.x-k8s.io"
	KindCluster            = "AWSCluster"
	VersionCluster         = "v1beta2"
	KindClusterIdentity    = "AWSClusterRoleIdentity"
	VersionClusterIdentity = "v1beta2"
)

func (c AWSClusterGetter) GetCluster(ctx context.Context, cli client.Client, name string, namespace string, baseDomain string, region string) (cluster.Cluster, error) {
	logger := log.FromContext(ctx)

	cluster, err := c.getClusterCR(ctx, cli, name, namespace)
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
	clusterIdentity, err := c.getClusterCRIdentiy(ctx, cli, clusterIdentityName, namespace)
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
		return nil, nil
	}

	return AWSCluster{
		Name:       name,
		Namespace:  namespace,
		BaseDomain: baseDomain,
		Region:     region,
		Role:       roleArn,
		Tags:       clusterTags,
	}, nil
}

func (c AWSClusterGetter) getClusterCR(ctx context.Context, cli client.Client, name string, namespace string) (*unstructured.Unstructured, error) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindCluster,
		Version: VersionCluster,
	})
	err := cli.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, cluster)
	return cluster, errors.WithStack(err)
}

func (c AWSClusterGetter) getClusterCRIdentiy(ctx context.Context, cli client.Client, clusterIdentityName string, namespace string) (*unstructured.Unstructured, error) {
	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   Group,
		Kind:    KindClusterIdentity,
		Version: VersionClusterIdentity,
	})

	err := cli.Get(ctx, client.ObjectKey{
		Name:      clusterIdentityName,
		Namespace: namespace,
	}, clusterIdentity)
	return clusterIdentity, errors.WithStack(err)
}

// AWSCluster implements Cluster Interface with AWS data
type AWSCluster struct {
	Name       string
	Namespace  string
	BaseDomain string
	Region     string
	Role       string
	Tags       map[string]string
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

func (c AWSCluster) GetRole() string {
	return c.Role
}

func (c AWSCluster) GetTags() map[string]string {
	return c.Tags
}
