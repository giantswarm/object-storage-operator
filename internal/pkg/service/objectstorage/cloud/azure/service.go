package azure

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

type AzureObjectStorageService struct {
}

func (s AzureObjectStorageService) NewAccessRoleService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster) (objectstorage.AccessRoleService, error) {
	return NewAzureAccessService(), nil
}

func (s AzureObjectStorageService) NewObjectStorageService(ctx context.Context, logger logr.Logger, cluster cluster.Cluster, client client.Client) (objectstorage.ObjectStorageService, error) {
	azurecluster, ok := cluster.(AzureCluster)
	if !ok {
		return nil, errors.New("Impossible to cast cluster into Azure cluster")
	}

	return NewAzureStorageService(logger, azurecluster, client)
}
