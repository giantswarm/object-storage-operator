package aws

import (
	"bytes"
	"context"
	"errors"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type S3ObjectStorageAdapter struct {
	s3Client             *s3.Client
	logger               logr.Logger
	cluster              AWSCluster
	bucketPolicyTemplate *template.Template
}

func NewS3Service(s3Client *s3.Client, logger logr.Logger, cluster AWSCluster) S3ObjectStorageAdapter {
	bucketPolicyTemplate, err := template.New("bucketPolicy").Parse(bucketPolicy)
	if err != nil {
		panic(err)
	}

	return S3ObjectStorageAdapter{
		s3Client:             s3Client,
		logger:               logger,
		cluster:              cluster,
		bucketPolicyTemplate: bucketPolicyTemplate,
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
			LocationConstraint: types.BucketLocationConstraint(s.cluster.GetRegion()),
		},
	})
	return err
}

func (s S3ObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	// First we need to empty the bucket
	in := &s3.ListObjectsV2Input{Bucket: &bucket.Spec.Name}
	for {
		// We retrieve a batch of objects
		out, err := s.s3Client.ListObjectsV2(ctx, in)
		if err != nil {
			return err
		}

		// We delete each object
		for _, item := range out.Contents {
			_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &bucket.Spec.Name,
				Key:    item.Key,
			})
			if err != nil {
				return err
			}
		}

		// If there are more objects, we need to continue
		if *out.IsTruncated {
			in.ContinuationToken = out.ContinuationToken
		} else {
			// No more objects in the bucket, we go out of the loop
			break
		}
	}

	// Then we can delete the bucket
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

	// Set the bucket policy (enforce encryption in transit)
	err = s.setBucketPolicy(ctx, bucket)
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
						Days: &bucket.Spec.ExpirationPolicy.Days,
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

func (s S3ObjectStorageAdapter) setBucketPolicy(ctx context.Context, bucket *v1alpha1.Bucket) error {
	var policy bytes.Buffer
	err := s.bucketPolicyTemplate.Execute(&policy, BucketPolicyData{
		AWSDomain:  awsDomain(s.cluster.Region),
		BucketName: bucket.Spec.Name,
	})
	if err != nil {
		return err
	}
	_, err = s.s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket.Spec.Name),
		Policy: aws.String(policy.String()),
	})
	return err
}

func (s S3ObjectStorageAdapter) setTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	tags := make([]types.Tag, 0)
	for _, t := range bucket.Spec.Tags {
		// We use this to avoid pointer issues in range loops.
		tag := t
		if tag.Key != "" && tag.Value != "" {
			tags = append(tags, types.Tag{Key: &tag.Key, Value: &tag.Value})
		}
	}
	for k, v := range s.cluster.GetTags() {
		// We use this to avoid pointer issues in range loops.
		key := k
		value := v
		if key != "" && value != "" {
			tags = append(tags, types.Tag{Key: &key, Value: &value})
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
