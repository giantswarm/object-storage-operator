package s3

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type Service struct {
	client      *s3.Client
	cloudRegion string
}

func NewService(ctx context.Context, cloudRegion string, roleArn string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cloudRegion))
	if err != nil {
		return nil, err
	}

	// Assume role
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
	cfg.Credentials = aws.NewCredentialsCache(credentials)

	return &Service{s3.NewFromConfig(cfg), cloudRegion}, nil
}

// BucketExists checks whether a bucket exists in the current account.
func (s Service) BucketExists(ctx context.Context, bucket *v1alpha1.Bucket) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
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

func (s Service) CreateBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket.Spec.Name),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(s.cloudRegion),
		},
		ObjectOwnership: types.ObjectOwnershipObjectWriter,
	})
	return err
}

func (s Service) DeleteBucket(ctx context.Context, bucket *v1alpha1.Bucket) error {
	_, err := s.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket.Spec.Name),
	})
	return err
}

func (s Service) SetBucketACL(ctx context.Context, bucket *v1alpha1.Bucket) error {
	acl := *bucket.Spec.Acl
	_, err := s.client.PutBucketAcl(ctx, &s3.PutBucketAclInput{
		Bucket: aws.String(bucket.Spec.Name),
		ACL:    types.BucketCannedACL(acl),
	})
	return err
}

func (s Service) SetLifecycleRules(ctx context.Context, bucket *v1alpha1.Bucket) error {
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
		_, err := s.client.PutBucketLifecycleConfiguration(ctx, input)
		return err
	}

	_, err := s.client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(bucket.Spec.Name),
	})
	return err

}

func (s Service) SetTags(ctx context.Context, bucket *v1alpha1.Bucket) error {
	if len(bucket.Spec.Tags) == 0 {
		_, err := s.client.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{
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

	_, err := s.client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucket.Spec.Name),
		Tagging: &types.Tagging{
			TagSet: tags,
		},
	})
	return err
}