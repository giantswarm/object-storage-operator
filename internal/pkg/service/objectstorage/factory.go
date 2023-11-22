package objectstorage

import (
	"context"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/go-logr/logr"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ObjectStorageServiceFactory
type ObjectStorageServiceFactory interface {
	NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (ObjectStorageService, error)
	NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (AccessRoleService, error)
}
