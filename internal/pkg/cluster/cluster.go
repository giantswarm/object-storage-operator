package cluster

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/flags"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ClusterGetter
type ClusterGetter interface {
	GetCluster(ctx context.Context, cli client.Client, managementCluster flags.ManagementCluster) (Cluster, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Cluster
type Cluster interface {
	GetName() string
	GetNamespace() string
	GetBaseDomain() string
	GetRegion() string
	GetRole() string
	GetTags() map[string]string
}
