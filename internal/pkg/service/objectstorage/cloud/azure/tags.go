package azure

import (
	"strings"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

func sanitizeTagKey(tagName string) string {
	return strings.ReplaceAll(tagName, "-", "_")
}

func (s AzureObjectStorageAdapter) getBucketTags(bucket *v1alpha1.Bucket) map[string]*string {
	tags := make(map[string]*string)
	for _, t := range bucket.Spec.Tags {
		// We use this to avoid pointer issues in range loops.
		tag := t
		if tag.Key != "" && tag.Value != "" {
			tags[sanitizeTagKey(tag.Key)] = &tag.Value
		}
	}
	for k, v := range s.cluster.GetTags() {
		// We use this to avoid pointer issues in range loops.
		key := k
		value := v
		if key != "" && value != "" {
			tags[sanitizeTagKey(key)] = &value
		}
	}
	return tags
}
