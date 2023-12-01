package objectstorage

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, bucket *v1alpha1.Bucket) (ObjectStorageService, error)
	NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (AccessRoleService, error)
}
