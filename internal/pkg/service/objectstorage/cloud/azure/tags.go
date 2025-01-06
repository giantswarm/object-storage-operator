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
	for _, tag := range bucket.Spec.Tags {
		if tag.Key != "" && tag.Value != "" {
			tags[sanitizeTagKey(tag.Key)] = &tag.Value
		}
	}
	for key, value := range s.cluster.GetTags() {
		if key != "" && value != "" {
			tags[sanitizeTagKey(key)] = &value
		}
	}
	return tags
}
