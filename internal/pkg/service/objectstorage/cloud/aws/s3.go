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
	createBucketConfiguration := types.CreateBucketConfiguration{}
	// If the region is us-east-1, then location needs to be null, FFS
	// https://github.com/aws/aws-sdk-go-v2/issues/1894
	if s.cluster.GetRegion() != "us-east-1" {
		createBucketConfiguration.LocationConstraint = types.BucketLocationConstraint(s.cluster.GetRegion())
	}

	_, err := s.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    aws.String(bucket.Spec.Name),
		CreateBucketConfiguration: &createBucketConfiguration,
	})
	return err
}

// UpdateBucket does nothing as we cannot update an s3 bucket
func (s S3ObjectStorageAdapter) UpdateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	return nil
}

func (s S3ObjectStorageAdapter) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	// First we need to empty the bucket
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket.Spec.Name),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		var objects []types.ObjectIdentifier
		for _, object := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: object.Key,
			})
		}

		if len(objects) != 0 {
			_, err = s.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket.Spec.Name),
				Delete: &types.Delete{
					Objects: objects,
				},
			})
			if err != nil {
				return err

			}
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
					Filter: &types.LifecycleRuleFilter{
						// Apply to all objects
						Prefix: aws.String(""),
					},
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
