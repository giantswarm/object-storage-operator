package cluster

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ClusterGetter
type ClusterGetter interface {
	GetCluster(ctx context.Context) (Cluster, error)
}

type Cluster interface {
	GetClient() client.Client
	GetName() string
	GetNamespace() string
	GetBaseDomain() string
	GetRegion() string
	GetTags() map[string]string
	GetCredentials() Credentials
}

type Credentials interface{}
