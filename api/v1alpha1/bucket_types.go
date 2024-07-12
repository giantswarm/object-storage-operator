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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BucketFinalizer      = "bucket.objectstorage.giantswarm.io"
	AzureSecretFinalizer = "giantswarm.io/object-storage-operator"
	ReclaimPolicyRetain  = "Retain"
	ReclaimPolicyDelete  = "Delete"
)

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	// Name is the name of the bucket to create.
	Name string `json:"name"`

	// Expiration policy on the objects in the bucket.
	// +optional
	ExpirationPolicy *BucketExpirationPolicy `json:"expirationPolicy,omitempty"`

	// Reclaim policy on the bucket.
	// +optional
	ReclaimPolicy string `json:"reclaimPolicy,omitempty"`

	// Access role that can be assumed to access the bucket
	AccessRole *BucketAccessRole `json:"accessRole,omitempty"`

	// Tags to add to the bucket.
	// +optional
	Tags []BucketTag `json:"tags,omitempty"`
}

// BucketAccessRole defines the bucket access role to create in the cloud account
type BucketAccessRole struct {
	// Name of the role to create
	RoleName string `json:"roleName"`

	// ExtraBucketNames is a list of bucket names to add to the role policy in case the role needs to be able to access multiple buckets.
	ExtraBucketNames []string `json:"extraBucketNames,omitempty"`

	// Name of the service account
	ServiceAccountName string `json:"serviceAccountName"`

	// Namespace of the service account
	ServiceAccountNamespace string `json:"serviceAccountNamespace"`
}

// BucketExpirationPolicy defines the expiration policy on all objects contained in the bucket
type BucketExpirationPolicy struct {
	// Days sets a number of days before the data expires
	Days int32 `json:"days"`
}

// BucketTag defines the type for bucket tags
type BucketTag struct {
	// Key is the key of the bucket tag to add to the bucket.
	Key string `json:"key"`

	// Key is the key of the bucket tag to add to the bucket.
	Value string `json:"value"`
}

// BucketStatus defines the observed state of Bucket
type BucketStatus struct {
	// BucketReady is a boolean condition to reflect the successful creation
	// of a bucket.
	BucketReady bool `json:"bucketReady,omitempty"`

	// BucketID is the unique id of the bucket.
	// +optional
	BucketID string `json:"bucketID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Bucket is the Schema for the buckets API
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BucketList contains a list of Bucket
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}
