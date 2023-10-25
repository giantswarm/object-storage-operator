package capa

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
	"github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/services/s3"
)

// CAPABucketReconciler reconciles Buckets in CAPA environments.
type CAPABucketReconciler struct {
	Client            client.Client
	ManagementCluster managementcluster.ManagementCluster
}

func (r CAPABucketReconciler) createS3Service(ctx context.Context) (*s3.Service, error) {
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
		return nil, errors.WithStack(err)
	}

	clusterIdentityName, found, err := unstructured.NestedString(cluster.Object, "spec", "identityRef", "name")
	if err != nil {
		logger.Error(err, "Identity name is not a string")
		return nil, errors.WithStack(err)
	}
	if !found || clusterIdentityName == "" {
		logger.Info("Missing identity, skipping")
		return nil, errors.New("missing management cluster identify")
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
		return nil, errors.WithStack(err)
	}

	roleArn, found, err := unstructured.NestedString(clusterIdentity.Object, "spec", "roleARN")
	if err != nil {
		logger.Error(err, "Role arn is not a string")
		return nil, errors.WithStack(err)
	}
	if !found {
		return nil, errors.New("missing role arn")
	}

	return s3.NewService(ctx, r.ManagementCluster.Region, roleArn)
}

func (r CAPABucketReconciler) Reconcile(ctx context.Context, bucket *v1alpha1.Bucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started reconciling S3 Bucket", "namespace", bucket.Namespace, "name", bucket.Name)
	defer logger.Info("Finished reconciling S3 Bucket", "namespace", bucket.Namespace, "name", bucket.Name)

	s3Service, err := r.createS3Service(ctx)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Could not create S3 service, %v", err))
		return ctrl.Result{}, err
	}

	// Handle deleted clusters
	if !bucket.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, s3Service, bucket)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, s3Service, bucket)
}

// reconcileCreate creates the s3 bucket.
func (r CAPABucketReconciler) reconcileNormal(ctx context.Context, s3Service *s3.Service, bucket *v1alpha1.Bucket) (ctrl.Result, error) {
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
	exists, err := s3Service.BucketExists(ctx, bucket)
	if err != nil {
		logger.Info(fmt.Sprintf("Either you don't have access to bucket %v or another error occurred. "+
			"Here's what happened: %v", bucket.Spec.Name, err))
		return ctrl.Result{}, errors.WithStack(err)
	}

	if exists {
		logger.Info(fmt.Sprintf("Bucket %v exists and you already own it.", bucket.Spec.Name))
	} else {
		logger.Info(fmt.Sprintf("Bucket %v is available, creating", bucket.Spec.Name))
		err = s3Service.CreateBucket(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket could not be created")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	if bucket.Spec.Acl != nil {
		logger.Info("Setting up bucket acl")
		err = s3Service.SetBucketACL(ctx, bucket)
		if err != nil {
			logger.Error(err, "Bucket ACL could not be set")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	logger.Info("Setting up lifecycle rule")
	// If expiration is not set, we remove all lifecycle rules
	err = s3Service.SetLifecycleRules(ctx, bucket)
	if err != nil {
		logger.Error(err, "Bucket lifecycle configuration could not be set")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Setting up bucket tags")
	err = s3Service.SetTags(ctx, bucket)
	if err != nil {
		logger.Error(err, "Tags could not be applied")
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger.Info("Bucket ready")
	originalBucket = bucket.DeepCopy()
	bucket.Status.BucketID = bucket.Spec.Name
	bucket.Status.BucketReady = true
	if err := r.Client.Status().Patch(ctx, bucket, client.MergeFrom(originalBucket)); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return ctrl.Result{}, nil
}

// reconcileDelete deletes the s3 bucket.
func (r CAPABucketReconciler) reconcileDelete(ctx context.Context, s3Service *s3.Service, bucket *v1alpha1.Bucket) error {
	logger := log.FromContext(ctx)

	logger.Info("Checking if bucket exists")
	_, err := s3Service.BucketExists(ctx, bucket)
	if err == nil {
		logger.Info("Bucket exists, deleting")
		err = s3Service.DeleteBucket(ctx, bucket)
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
