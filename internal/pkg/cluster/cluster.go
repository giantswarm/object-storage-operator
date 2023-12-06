package cluster

import (
	"context"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ClusterGetter
type ClusterGetter interface {
	GetCluster(ctx context.Context) (Cluster, error)
}

type Cluster interface {
	GetName() string
	GetNamespace() string
	GetBaseDomain() string
	GetRegion() string
	GetTags() map[string]string
	GetCredentials() Credentials
}

type Credentials interface{}
