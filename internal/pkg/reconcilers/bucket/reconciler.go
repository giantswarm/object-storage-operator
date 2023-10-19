package bucket

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type BucketReconciler interface {
	Reconcile(ctx context.Context, bucket *v1alpha1.Bucket) (ctrl.Result, error)
}
