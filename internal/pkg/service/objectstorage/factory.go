package objectstorage

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, client client.Client) (ObjectStorageService, error)
	NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (AccessRoleService, error)
}
