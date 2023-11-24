package cluster

import (
	"context"

	corev1 "k8s.io/api/core/v1"
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
	GetRole() string
	GetTags() map[string]string
	GetSubscriptionID() string
	GetTypeIdentity() string
	GetClientID() string
	GetTenantID() string
	GetSecretRef() corev1.Secret
}
