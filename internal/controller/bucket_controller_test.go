package controller_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	"github.com/giantswarm/object-storage-operator/internal/controller"
	"github.com/giantswarm/object-storage-operator/internal/pkg/managementcluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/service/objectstorage/objectstoragefakes"
)

var _ = Describe("Bucket Reconciler", func() {
	const (
		BucketName      string = "my-bucket-name"
		BucketNamespace string = "default"
	)

	var (
		ctx context.Context

		reconciler   controller.BucketReconciler
		reconcileErr error

		fakeClient           client.Client
		serviceFactory       objectstoragefakes.FakeObjectStorageServiceFactory
		objectStorageService objectstoragefakes.FakeObjectStorageService
		accessRoleService    objectstoragefakes.FakeAccessRoleService
		bucketKey            = client.ObjectKey{
			Name:      BucketName,
			Namespace: BucketNamespace,
		}
	)

	// creates the dummy bucket and clients
	BeforeEach(func() {
		SetDefaultEventuallyPollingInterval(time.Second)
		SetDefaultEventuallyTimeout(time.Second * 90)

		ctx = context.Background()

		fakeClient = fake.NewClientBuilder().WithStatusSubresource(&v1alpha1.Bucket{}).Build()

		serviceFactory = objectstoragefakes.FakeObjectStorageServiceFactory{}
		objectStorageService = objectstoragefakes.FakeObjectStorageService{}
		accessRoleService = objectstoragefakes.FakeAccessRoleService{}
		serviceFactory.NewS3ServiceReturns(&objectStorageService, nil)
		serviceFactory.NewIAMServiceReturns(&accessRoleService, nil)
	})

	var _ = Describe("CAPA", func() {
		// creates the reconciler
		BeforeEach(func() {
			reconciler = controller.BucketReconciler{
				Client:                      fakeClient,
				ObjectStorageServiceFactory: &serviceFactory,
				ManagementCluster: managementcluster.ManagementCluster{
					Name:      "test-mc",
					Namespace: "giantswarm",
					Provider:  "capa",
					Region:    "eu-central-1",
				},
			}
		})

		JustBeforeEach(func() {
			// starts the reconciler
			request := ctrl.Request{NamespacedName: bucketKey}
			_, reconcileErr = reconciler.Reconcile(ctx, request)
		})

		When("reconciling a missing bucket", func() {
			It("does nothing", func() {
				Expect(reconcileErr).ToNot(HaveOccurred())
				var existingBucket v1alpha1.Bucket
				_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
				Expect(existingBucket.Finalizers).To(BeEmpty())
			})
		})

		When("the management cluster is in error", func() {
			BeforeEach(func() {
				// creates dummy bucket
				bucket := v1alpha1.Bucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      BucketName,
						Namespace: BucketNamespace,
					},
					Spec: v1alpha1.BucketSpec{
						Name: BucketName,
					},
					Status: v1alpha1.BucketStatus{},
				}
				_ = fakeClient.Create(ctx, &bucket)
			})

			When("the management cluster CR is missing", func() {
				It("fails", func() {
					Expect(reconcileErr).To(HaveOccurred())
					var existingBucket v1alpha1.Bucket
					_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
					Expect(existingBucket.Finalizers).To(BeEmpty())
				})
			})

			When("the management cluster has no identity set", func() {
				BeforeEach(func() {
					cluster := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"kind":       "AWSCluster",
							"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
							"metadata": map[string]interface{}{
								"name":      reconciler.ManagementCluster.Name,
								"namespace": reconciler.ManagementCluster.Namespace,
							},
							"spec": map[string]interface{}{
								"identityRef": map[string]interface{}{},
							},
						},
					}
					_ = fakeClient.Create(ctx, cluster)
				})
				It("fails", func() {
					Expect(reconcileErr).To(HaveOccurred())
					var existingBucket v1alpha1.Bucket
					_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
					Expect(existingBucket.Finalizers).To(BeEmpty())
				})
			})

			When("management cluster identity is missing", func() {
				BeforeEach(func() {
					cluster := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"kind":       "AWSCluster",
							"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
							"metadata": map[string]interface{}{
								"name":      reconciler.ManagementCluster.Name,
								"namespace": reconciler.ManagementCluster.Namespace,
							},
							"spec": map[string]interface{}{
								"identityRef": map[string]interface{}{
									"name": reconciler.ManagementCluster.Name,
								},
							},
						},
					}
					_ = fakeClient.Create(ctx, cluster)
				})
				It("fails", func() {
					Expect(reconcileErr).To(HaveOccurred())
					var existingBucket v1alpha1.Bucket
					_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
					Expect(existingBucket.Finalizers).To(BeEmpty())
				})
			})

			When("management cluster identity has no role arn", func() {
				BeforeEach(func() {
					clusterIdentity := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"kind":       "AWSClusterRoleIdentity",
							"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
							"metadata": map[string]interface{}{
								"name":      reconciler.ManagementCluster.Name,
								"namespace": reconciler.ManagementCluster.Namespace,
							},
							"spec": map[string]interface{}{},
						},
					}
					_ = fakeClient.Create(ctx, clusterIdentity)

					cluster := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"kind":       "AWSCluster",
							"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
							"metadata": map[string]interface{}{
								"name":      reconciler.ManagementCluster.Name,
								"namespace": reconciler.ManagementCluster.Namespace,
							},
							"spec": map[string]interface{}{
								"identityRef": map[string]interface{}{
									"name": reconciler.ManagementCluster.Name,
								},
							},
						},
					}
					_ = fakeClient.Create(ctx, cluster)
				})
				It("fails", func() {
					Expect(reconcileErr).To(HaveOccurred())
					var existingBucket v1alpha1.Bucket
					_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
					Expect(existingBucket.Finalizers).To(BeEmpty())
				})
			})
		})

		When("the management cluster has an identity set", func() {
			// creates a dummy management cluster and management cluster identity
			BeforeEach(func() {
				clusterIdentity := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind":       "AWSClusterRoleIdentity",
						"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
						"metadata": map[string]interface{}{
							"name":      reconciler.ManagementCluster.Name,
							"namespace": reconciler.ManagementCluster.Namespace,
						},
						"spec": map[string]interface{}{
							"roleARN": "role",
						},
					},
				}
				_ = fakeClient.Create(ctx, clusterIdentity)

				cluster := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind":       "AWSCluster",
						"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
						"metadata": map[string]interface{}{
							"name":      reconciler.ManagementCluster.Name,
							"namespace": reconciler.ManagementCluster.Namespace,
						},
						"spec": map[string]interface{}{
							"identityRef": map[string]interface{}{
								"name": reconciler.ManagementCluster.Name,
							},
						},
					},
				}
				_ = fakeClient.Create(ctx, cluster)
			})

			When("the bucket is being created/updated", func() {
				BeforeEach(func() {
					// creates dummy bucket
					bucket := v1alpha1.Bucket{
						ObjectMeta: metav1.ObjectMeta{
							Name:      BucketName,
							Namespace: BucketNamespace,
						},
						Spec: v1alpha1.BucketSpec{
							Name: BucketName,
						},
						Status: v1alpha1.BucketStatus{},
					}
					_ = fakeClient.Create(ctx, &bucket)
				})

				When("reconciling a s3 bucket we do not own", func() {
					expectedError := errors.New("bucket not owned")
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(false, expectedError)
					})

					It("failed", func() {
						Expect(reconcileErr).To(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
					})
				})

				When("reconciling a new s3 bucket", func() {
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(false, nil)
					})

					It("was created", func() {
						Expect(reconcileErr).ToNot(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.CreateBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.ConfigureBucketCallCount()).To(Equal(1))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
						Expect(existingBucket.Status.BucketID).To(Equal(BucketName))
						Expect(existingBucket.Status.BucketReady).To(BeTrue())
					})
				})

				When("reconciling an exiting s3 bucket", func() {
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(true, nil)
					})
					It("was updated", func() {
						Expect(reconcileErr).ToNot(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.ConfigureBucketCallCount()).To(Equal(1))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
						Expect(existingBucket.Status.BucketID).To(Equal(BucketName))
						Expect(existingBucket.Status.BucketReady).To(BeTrue())
					})
				})

				When("there is an error trying to create the bucket being reconciled", func() {
					expectedError := errors.New("failed creating the Bucket")

					BeforeEach(func() {
						objectStorageService.CreateBucketReturns(expectedError)
					})

					It("returns the error", func() {
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
						Expect(reconcileErr).To(HaveOccurred())
						Expect(reconcileErr).Should(MatchError(expectedError))
					})
				})

				When("there is an error trying to configure the bucket being reconciled", func() {
					expectedError := errors.New("failed configuring the Bucket")

					BeforeEach(func() {
						objectStorageService.ConfigureBucketReturns(expectedError)
					})

					It("returns the error", func() {
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
						Expect(reconcileErr).To(HaveOccurred())
						Expect(reconcileErr).Should(MatchError(expectedError))
					})
				})
			})

			When("the bucket is being deleted", func() {
				BeforeEach(func() {
					// creates dummy bucket in deleting state
					var gracePeriod int64 = 120
					bucket := v1alpha1.Bucket{
						ObjectMeta: metav1.ObjectMeta{
							Name:      BucketName,
							Namespace: BucketNamespace,
							Finalizers: []string{
								v1alpha1.BucketFinalizer,
							},
						},
						Spec: v1alpha1.BucketSpec{
							Name: BucketName,
						},
						Status: v1alpha1.BucketStatus{},
					}
					_ = fakeClient.Create(ctx, &bucket)
					_ = fakeClient.Delete(ctx, &bucket, &client.DeleteOptions{GracePeriodSeconds: &gracePeriod})
				})

				When("deleting a bucket is failing", func() {
					expectedError := errors.New("bucket could not be deleted")
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(true, nil)
						objectStorageService.DeleteBucketReturns(expectedError)
					})

					It("was not deleted", func() {
						Expect(reconcileErr).To(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.DeleteBucketCallCount()).To(Equal(1))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).To(ContainElement(v1alpha1.BucketFinalizer))
					})
				})

				When("deleting a bucket that does not exists", func() {
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(false, nil)
					})

					It("was free of its finalizer", func() {
						Expect(reconcileErr).ToNot(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.DeleteBucketCallCount()).To(Equal(0))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).ToNot(ContainElement(v1alpha1.BucketFinalizer))
					})
				})

				When("deleting a bucket that does not exists", func() {
					BeforeEach(func() {
						objectStorageService.ExistsBucketReturns(true, nil)
					})
					It("was deleted", func() {
						Expect(reconcileErr).ToNot(HaveOccurred())
						Expect(objectStorageService.ExistsBucketCallCount()).To(Equal(1))
						Expect(objectStorageService.DeleteBucketCallCount()).To(Equal(1))
						var existingBucket v1alpha1.Bucket
						_ = fakeClient.Get(ctx, bucketKey, &existingBucket)
						Expect(existingBucket.Finalizers).ToNot(ContainElement(v1alpha1.BucketFinalizer))
					})
				})
			})
		})
	})

	var _ = Describe("Unknown provider", func() {
		// creates the reconciler
		BeforeEach(func() {
			// creates dummy bucket
			bucket := v1alpha1.Bucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BucketName,
					Namespace: BucketNamespace,
				},
				Spec: v1alpha1.BucketSpec{
					Name: BucketName,
				},
				Status: v1alpha1.BucketStatus{},
			}
			_ = fakeClient.Create(ctx, &bucket)

			reconciler = controller.BucketReconciler{
				Client:                      fakeClient,
				ObjectStorageServiceFactory: &serviceFactory,
				ManagementCluster: managementcluster.ManagementCluster{
					Name:      "test-mc",
					Namespace: "giantswarm",
					Provider:  "unknown",
					Region:    "eu-central-1",
				},
			}

			request := ctrl.Request{NamespacedName: bucketKey}
			_, reconcileErr = reconciler.Reconcile(ctx, request)
		})

		When("reconciling", func() {
			It("fails", func() {
				Expect(reconcileErr).To(HaveOccurred())
			})
		})
	})

})
