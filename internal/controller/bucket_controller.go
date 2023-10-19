/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	managementcluster "github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/reconcilers/bucket"
	"github.com/giantswarm/object-storage-operator/internal/pkg/reconcilers/bucket/capa"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	*runtime.Scheme
	managementcluster.ManagementCluster
}

//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Bucket", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished reconciling Bucket", "namespace", req.Namespace, "name", req.Name)

	// Get the bucket that we are reconciling
	reconciledBucket := &v1alpha1.Bucket{}
	err := r.Client.Get(ctx, req.NamespacedName, reconciledBucket)
	if apierrors.IsNotFound(err) {
		logger.Info("Bucket no longer exists")
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Create the correct reconciler based on the provider
	var bucketReconciler bucket.BucketReconciler
	switch r.ManagementCluster.Provider {
	case "capa":
		bucketReconciler = capa.CAPABucketReconciler{
			Client:            r.Client,
			ManagementCluster: r.ManagementCluster,
		}
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported provider %s", r.ManagementCluster.Provider)
	}

	return bucketReconciler.Reconcile(ctx, reconciledBucket)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Bucket{}).
		Complete(r)
}
