package managementcluster

import (
	"context"

	objectstorage "github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
	"github.com/go-logr/logr"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Cluster
type Cluster interface {
	NewObjectStorageService(ctx context.Context, logger logr.Logger) (objectstorage.ObjectStorageService, error)
	NewAccessRoleService(ctx context.Context, logger logr.Logger) (objectstorage.AccessRoleService, error)
}

type ManagementCluster struct {
	BaseDomain           string
	Name                 string
	Namespace            string
	Provider             string
	Region               string
	ObjectStorageService objectstorage.ObjectStorageService
	AccessRoleService    objectstorage.AccessRoleService
}
