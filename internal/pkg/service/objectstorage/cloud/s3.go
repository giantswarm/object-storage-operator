package cloud

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type S3ObjectStorageAdapter struct {
	s3Client    *s3.Client
	cloudRegion string
}

func NewS3Service(s3Client *s3.Client, cloudRegion string) S3ObjectStorageAdapter {
	return S3ObjectStorageAdapter{
		s3Client:    s3Client,
		cloudRegion: cloudRegion,
	}
}
func (s S3ObjectStorageAdapter) ExistsBucket(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	_, err := s.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket.Spec.Name),
	})
	exists := true
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NotFound:
				exists = false
				err = nil
			}
		}
	}

	return exists, err
}

func (s S3ObjectStorageAdapter) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket.Spec.Name),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(s.cloudRegion),
		},
	})
	return err
}

func (s S3ObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket.Spec.Name),
	})
	return err
}

func (s S3ObjectStorageAdapter) ConfigureBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	var err error
	// If expiration is not set, we remove all lifecycle rules
	err = s.setLifecycleRules(ctx, bucket)
	if err != nil {
		return err
	}

	err = s.setTags(ctx, bucket)
	return err
}

func (s S3ObjectStorageAdapter) setLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
	if bucket.Spec.ExpirationPolicy != nil {
		enabledRuleStatus := types.ExpirationStatusEnabled
		lifecycleConfiguration := types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					Status: enabledRuleStatus,
					ID:     aws.String("Expiration"),
					Filter: &types.LifecycleRuleFilterMemberPrefix{Value: ""},
					Expiration: &types.LifecycleExpiration{
						Days: bucket.Spec.ExpirationPolicy.Days,
					},
				},
			},
		}
		input := &s3.PutBucketLifecycleConfigurationInput{
			Bucket:                 aws.String(bucket.Spec.Name),
			LifecycleConfiguration: &lifecycleConfiguration,
		}
		_, err := s.s3Client.PutBucketLifecycleConfiguration(ctx, input)
		return err
	}

	_, err := s.s3Client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(bucket.Spec.Name),
	})
	return err

}

func (s S3ObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	if len(bucket.Spec.Tags) == 0 {
		_, err := s.s3Client.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{
			Bucket: aws.String(bucket.Spec.Name),
		})
		return err
	}

	tags := make([]types.Tag, 0)
	for _, t := range bucket.Spec.Tags {
		// We use this to avoid pointer issues in range loops.
		tag := t
		if tag.Key != "" && tag.Value != "" {
			tags = append(tags, types.Tag{
				Key:   &tag.Key,
				Value: &tag.Value,
			})
		}
	}

	_, err := s.s3Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucket.Spec.Name),
		Tagging: &types.Tagging{
			TagSet: tags,
		},
	})
	return err
}
