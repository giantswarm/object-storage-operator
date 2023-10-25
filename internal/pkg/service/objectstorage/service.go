package objectstorage

import (
	"context"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type ObjectStorageService interface {
	// Configure all bucket related configurations (ACL, Tags, ...)
	ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error
	CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error
	DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error
	// Exists checks whether a bucket exists in the current account.
	ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error)
}
