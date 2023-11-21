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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	managementcluster "github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	managementcluster.ManagementCluster
}

//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the bucket closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Bucket")
	defer logger.Info("Finished reconciling Bucket")

	bucket := &v1alpha1.Bucket{}
	err := r.Client.Get(ctx, req.NamespacedName, bucket)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	logger.WithValues("bucket", bucket.Spec.Name)

	// Create the correct service implementation based on the provider
	var objectStorageService objectstorage.ObjectStorageService
	var accessRoleService objectstorage.AccessRoleService
	switch r.ManagementCluster.Provider {
	case "capa":
		var cluster = managementcluster.CAPACluster{
			ManagementCluster: r.ManagementCluster,
		}
		cluster.RoleToAssume, err = cluster.GetRoleArn(ctx, req, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		objectStorageService, err = cluster.NewObjectStorageService(ctx, logger)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		accessRoleService, err = cluster.NewAccessRoleService(ctx, logger)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	default:
		return ctrl.Result{}, errors.New(fmt.Sprintf("unsupported provider %s", r.ManagementCluster.Provider))
	}

	// Handle deleted clusters
	if !bucket.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, objectStorageService, accessRoleService, bucket)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, objectStorageService, accessRoleService, bucket)
}

// reconcileCreate creates the s3 bucket.
func (r BucketReconciler) reconcileNormal(ctx context.Context, objectStorageService objectstorage.ObjectStorageService, accessRoleService objectstorage.AccessRoleService, bucket *v1alpha1.Bucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	originalBucket := bucket.DeepCopy()
	// If the Bucket doesn't have our finalizer, add it.
	if controllerutil.AddFinalizer(bucket, v1alpha1.BucketFinalizer) {
		// Register the finalizer immediately to avoid orphaning AWS resources on delete
		if err := r.Client.Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	logger.Info("Checking if bucket exists")
	exists, err := objectStorageService.ExistsBucket(ctx, bucket)
	if err != nil {
		logger.Error(err, "Either you don't have access to the bucket or another error occurred")
		return ctrl.Result{}, errors.WithStack(err)
	} else if !exists {
		logger.Info("Bucket is available, creating")
		err = objectStorageService.CreateBucket(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket could not be created")
			return ctrl.Result{}, errors.WithStack(err)
		}
	} else {
		logger.Info("Bucket exists and you already own it.")
	}

	logger.Info("Configuring bucket settings")
	// If expiration is not set, we remove all lifecycle rules
	err = objectStorageService.ConfigureBucket(ctx, bucket)
	if err != nil {
		logger.Error(err, "Bucket could not be configured")
		return ctrl.Result{}, errors.WithStack(err)
	}

	originalBucket = bucket.DeepCopy()
	bucket.Status = v1alpha1.BucketStatus{
		BucketID:    bucket.Spec.Name,
		BucketReady: true,
	}

	if err = r.Client.Status().Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	logger.Info("Bucket ready")

	if bucket.Spec.AccessRole != nil && bucket.Spec.AccessRole.RoleName != "" {
		logger.Info("Creating bucket access role")
		err = accessRoleService.ConfigureRole(ctx, bucket)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("Bucket access role created")
	}
	return ctrl.Result{}, nil
}

// reconcileDelete deletes the s3 bucket.
func (r BucketReconciler) reconcileDelete(ctx context.Context, objectStorageService objectstorage.ObjectStorageService, accessRoleService objectstorage.AccessRoleService, bucket *v1alpha1.Bucket) error {
	logger := log.FromContext(ctx)

	logger.Info("Checking if bucket exists")
	exists, err := objectStorageService.ExistsBucket(ctx, bucket)
	if err == nil && exists {
		logger.Info("Bucket exists, deleting")
		err = objectStorageService.DeleteBucket(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket could not be deleted")
			return errors.WithStack(err)
		}
	}

	logger.Info("Bucket deleted")

	if bucket.Spec.AccessRole != nil && bucket.Spec.AccessRole.RoleName != "" {
		logger.Info("Deleting bucket access role")
		err = accessRoleService.DeleteRole(ctx, bucket)
		if err != nil {
			return errors.WithStack(err)
		}
		logger.Info("Bucket access role deleted")
	}

	originalBucket := bucket.DeepCopy()
	// Bucket is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(bucket, v1alpha1.BucketFinalizer)
	return r.Client.Patch(ctx, bucket, client.MergeFrom(originalBucket))

}

// SetupWithManager sets up the controller with the Manager.
func (r BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Bucket{}).
		Complete(r)
}
