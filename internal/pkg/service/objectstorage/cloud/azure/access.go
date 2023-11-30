package azure

import (
	"context"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type AzureAccessServiceAdapter struct {
}

func NewAzureAccessService() AzureAccessServiceAdapter {
	return AzureAccessServiceAdapter{}
}

func (s AzureAccessServiceAdapter) ConfigureRole(ctx context.Context, bucket *v1alpha1.Bucket, additionalTags map[string]string) error {
	return nil
}

func (s AzureAccessServiceAdapter) DeleteRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return nil
}
