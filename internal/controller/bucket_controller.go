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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	managementcluster "github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/cloud"
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
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started reconciling Bucket", "namespace", req.Namespace, "name", req.Name)
	defer logger.Info("Finished reconciling Bucket", "namespace", req.Namespace, "name", req.Name)

	// Get the bucket that we are reconciling
	bucket := &v1alpha1.Bucket{}
	err := r.Client.Get(ctx, req.NamespacedName, bucket)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	// Create the correct reconciler based on the provider
	var service objectstorage.ObjectStorageService
	switch r.ManagementCluster.Provider {
	case "capa":
		roleArn, err := r.getRoleArn(ctx)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		service, err = cloud.NewS3Service(ctx, r.ManagementCluster.Region, roleArn)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported provider %s", r.ManagementCluster.Provider)
	}

	// Handle deleted clusters
	if !bucket.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, service, bucket)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, service, bucket)
}

// reconcileCreate creates the s3 bucket.
func (r *BucketReconciler) reconcileNormal(ctx context.Context, service objectstorage.ObjectStorageService, bucket *v1alpha1.Bucket) (ctrl.Result, error) {
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
	exists, err := service.ExistsBucket(ctx, bucket)
	if err != nil {
		logger.Info(fmt.Sprintf("Either you don't have access to bucket %v or another error occurred. "+
			"Here's what happened: %v", bucket.Spec.Name, err))
		return ctrl.Result{}, errors.WithStack(err)
	} else if !exists {
		logger.Info(fmt.Sprintf("Bucket %v is available, creating", bucket.Spec.Name))
		err = service.CreateBucket(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket could not be created")
			return ctrl.Result{}, errors.WithStack(err)
		}
	} else {
		logger.Info(fmt.Sprintf("Bucket %v exists and you already own it.", bucket.Spec.Name))
	}

	logger.Info("Configuring bucket settings")
	// If expiration is not set, we remove all lifecycle rules
	err = service.ConfigureBucket(ctx, bucket)
	if err != nil {
		logger.Error(err, "Bucket could not be configured")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Bucket ready")
	originalBucket = bucket.DeepCopy()
	bucket.Status = v1alpha1.BucketStatus{
		BucketID:    bucket.Spec.Name,
		BucketReady: true,
	}
	if err := r.Client.Status().Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

// reconcileDelete deletes the s3 bucket.
func (r *BucketReconciler) reconcileDelete(ctx context.Context, service objectstorage.ObjectStorageService, bucket *v1alpha1.Bucket) error {
	logger := log.FromContext(ctx)

	logger.Info("Checking if bucket exists")
	_, err := service.ExistsBucket(ctx, bucket)
	if err == nil {
		logger.Info("Bucket exists, deleting")
		err = service.DeleteBucket(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket could not be deleted")
			return errors.WithStack(err)
		}
	}

	logger.Info("Bucket deleted")
	originalBucket := bucket.DeepCopy()
	// Bucket is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(bucket, v1alpha1.BucketFinalizer)
	return r.Client.Patch(ctx, bucket, client.MergeFrom(originalBucket))
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Bucket{}).
		Complete(r)
}

func (r *BucketReconciler) getRoleArn(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Kind:    "AWSCluster",
		Version: "v1beta2",
	})

	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      r.ManagementCluster.Name,
		Namespace: r.ManagementCluster.Namespace,
	}, cluster)
	if err != nil {
		logger.Error(err, "Missing management cluster AWSCluster CR")
		return "", errors.WithStack(err)
	}

	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return "", errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return "", errors.New("missing management cluster identify")
	}

	clusterIdentity := &unstructured.Unstructured{}
	clusterIdentity.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Kind:    "AWSClusterRoleIdentity",
		Version: "v1beta2",
	})

	err = r.Client.Get(ctx, client.ObjectKey{
		Name:      clusterIdentityName,
		Namespace: cluster.GetNamespace(),
	}, clusterIdentity)
	if err != nil {
		logger.Error(err, "Missing management cluster identity AWSClusterRoleIdentity CR")
		return "", errors.WithStack(err)
	}

	roleArn, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "roleARN")
	if err != nil {
		logger.Error(err, "Role arn is not a string")
		return "", errors.WithStack(err)
	}
	if !found {
		return "", errors.New("missing role arn")
	}
	return roleArn, nil
}
