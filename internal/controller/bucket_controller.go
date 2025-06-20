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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/flags"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	cluster.ClusterGetter
	objectstorage.ObjectStorageServiceFactory
	flags.ManagementCluster
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
	err := r.Get(ctx, req.NamespacedName, bucket)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.WithValues("bucket", bucket.Spec.Name)

	cluster, err := r.GetCluster(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get cluster for bucket %s: %w", bucket.Spec.Name, err)
	}
	objectStorageService, err := r.NewObjectStorageService(ctx, logger, cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create object storage service for bucket %s: %w", bucket.Spec.Name, err)
	}
	accessRoleService, err := r.NewAccessRoleService(ctx, logger, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create access role service for bucket %s: %w", bucket.Spec.Name, err)
	}

	// Handle deleted clusters
	if !bucket.DeletionTimestamp.IsZero() {
		err := r.reconcileDelete(ctx, objectStorageService, accessRoleService, bucket)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete bucket %s: %w", bucket.Spec.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, objectStorageService, accessRoleService, bucket)
}

// reconcileCreate creates the bucket.
func (r BucketReconciler) reconcileNormal(ctx context.Context, objectStorageService objectstorage.ObjectStorageService, accessRoleService objectstorage.AccessRoleService, bucket *v1alpha1.Bucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	originalBucket := bucket.DeepCopy()
	// If the Bucket doesn't have our finalizer, add it.
	if controllerutil.AddFinalizer(bucket, v1alpha1.BucketFinalizer) {
		// Register the finalizer immediately to avoid orphaning AWS resources on delete
		if err := r.Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer to bucket %s: %w", bucket.Spec.Name, err)
		}
	}

	logger.Info("Checking if bucket exists")
	exists, err := objectStorageService.ExistsBucket(ctx, bucket)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if bucket %s exists: %w", bucket.Spec.Name, err)
	} else if !exists {
		logger.Info("Bucket is available, creating")
		err = objectStorageService.CreateBucket(ctx, bucket)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create bucket %s: %w", bucket.Spec.Name, err)
		}
	} else {
		logger.Info("Bucket exists and you already own it, let's update it")
		err = objectStorageService.UpdateBucket(ctx, bucket)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update bucket %s: %w", bucket.Spec.Name, err)
		}
	}

	logger.Info("Configuring bucket settings")
	// If expiration is not set, we remove all lifecycle rules
	err = objectStorageService.ConfigureBucket(ctx, bucket)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to configure bucket %s: %w", bucket.Spec.Name, err)
	}

	originalBucket = bucket.DeepCopy()
	bucket.Status = v1alpha1.BucketStatus{
		BucketID:    bucket.Spec.Name,
		BucketReady: true,
	}

	if err = r.Client.Status().Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status for bucket %s: %w", bucket.Spec.Name, err)
	}
	logger.Info("Bucket ready")

	if bucket.Spec.AccessRole != nil && bucket.Spec.AccessRole.RoleName != "" {
		logger.Info("Creating bucket access role")
		err = accessRoleService.ConfigureRole(ctx, bucket)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to configure access role for bucket %s: %w", bucket.Spec.Name, err)
		}
		logger.Info("Bucket access role created")
	}
	return ctrl.Result{}, nil
}

// reconcileDelete deletes the bucket.
func (r BucketReconciler) reconcileDelete(ctx context.Context, objectStorageService objectstorage.ObjectStorageService, accessRoleService objectstorage.AccessRoleService, bucket *v1alpha1.Bucket) error {
	logger := log.FromContext(ctx)

	logger.Info("Checking if bucket exists")
	exists, err := objectStorageService.ExistsBucket(ctx, bucket)
	if err == nil && exists {
		switch bucket.Spec.ReclaimPolicy {
		case v1alpha1.ReclaimPolicyDelete:
			logger.Info("Reclaim policy is set to delete, deleting bucket")

			logger.Info("Bucket exists, deleting")
			err = objectStorageService.DeleteBucket(ctx, bucket)
			if err != nil {
				return fmt.Errorf("failed to delete bucket %s: %w", bucket.Spec.Name, err)
			}
			logger.Info("Bucket deleted")

			if bucket.Spec.AccessRole != nil && bucket.Spec.AccessRole.RoleName != "" {
				logger.Info("Deleting bucket access role")
				err = accessRoleService.DeleteRole(ctx, bucket)
				if err != nil {
					return fmt.Errorf("failed to delete access role for bucket %s: %w", bucket.Spec.Name, err)
				}
				logger.Info("Bucket access role deleted")
			} else {
				logger.Info("Bucket access role not found, skipping deletion")
			}

			// Remove the finalizer.
			originalBucket := bucket.DeepCopy()
			controllerutil.RemoveFinalizer(bucket, v1alpha1.BucketFinalizer)
			return r.Patch(ctx, bucket, client.MergeFrom(originalBucket))
		case v1alpha1.ReclaimPolicyRetain:
			logger.Info("Reclaim policy is set to retain, not deleting bucket")
		default:
			logger.Info("Reclaim policy is the default one (retain), not deleting bucket")
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Bucket{}).
		Complete(r)
}
