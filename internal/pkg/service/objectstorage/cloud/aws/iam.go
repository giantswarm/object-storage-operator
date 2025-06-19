package aws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

type IAMAccessRoleServiceAdapter struct {
	iamClient           *iam.Client
	logger              logr.Logger
	accountId           string
	cluster             AWSCluster
	trustIdentityPolicy *template.Template
	rolePolicy          *template.Template
}

func NewIamService(iamClient *iam.Client, logger logr.Logger, accountId string, cluster AWSCluster) IAMAccessRoleServiceAdapter {
	trustIdentityPolicy, err := template.New("trustIdentityPolicy").Parse(trustIdentityPolicy)
	if err != nil {
		panic(err)
	}
	rolePolicy, err := template.New("rolePolicy").Parse(rolePolicy)
	if err != nil {
		panic(err)
	}
	return IAMAccessRoleServiceAdapter{
		iamClient:           iamClient,
		logger:              logger,
		accountId:           accountId,
		cluster:             cluster,
		trustIdentityPolicy: trustIdentityPolicy,
		rolePolicy:          rolePolicy,
	}
}

func (s IAMAccessRoleServiceAdapter) getRole(ctx context.Context, roleName string) (*types.Role, error) {
	output, err := s.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})

	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NoSuchEntityException:
				s.logger.Info("IAM role does not exist")
				return nil, nil
			default:
				return nil, fmt.Errorf("failed to fetch IAM role %s: %w", roleName, err)
			}
		}
		return nil, fmt.Errorf("failed to get IAM role %s: %w", roleName, err)
	}
	return output.Role, nil
}

func (s IAMAccessRoleServiceAdapter) irsaDomain() string {
	if isChinaRegion(s.cluster.Region) {
		return fmt.Sprintf("s3.%s.amazonaws.com.cn/%s-g8s-%s-oidc-pod-identity-v3", s.cluster.Region, s.accountId, s.cluster.GetName())
	} else {
		return fmt.Sprintf("irsa.%s.%s", s.cluster.Name, s.cluster.GetBaseDomain())
	}
}

func (s IAMAccessRoleServiceAdapter) ConfigureRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	roleName := bucket.Spec.AccessRole.RoleName
	role, err := s.getRole(ctx, roleName)
	if err != nil {
		return err
	}

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

	var trustPolicy bytes.Buffer
	err = s.trustIdentityPolicy.Execute(&trustPolicy, TrustIdentityPolicyData{
		AccountId:               s.accountId,
		AWSDomain:               awsDomain(s.cluster.Region),
		CloudFrontDomain:        s.irsaDomain(),
		ServiceAccountName:      bucket.Spec.AccessRole.ServiceAccountName,
		ServiceAccountNamespace: bucket.Spec.AccessRole.ServiceAccountNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to execute trust identity policy template for role %s: %w", roleName, err)
	}

	if role == nil {
		_, err := s.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
			RoleName:                 aws.String(roleName),
			AssumeRolePolicyDocument: aws.String(trustPolicy.String()),
			Description:              aws.String("Role for Giant Swarm managed Loki"),
			Tags:                     tags,
		})
		if err != nil {
			return fmt.Errorf("failed to create IAM role %s: %w", roleName, err)
		}
		s.logger.Info("IAM Role created")
	} else {
		_, err = s.iamClient.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
			RoleName:       aws.String(roleName),
			PolicyDocument: aws.String(trustPolicy.String()),
		})
		if err != nil {
			return fmt.Errorf("failed to update assume role policy for IAM role %s: %w", roleName, err)
		}

		// Update tags (need to untag with existing keys then retag)
		if !reflect.DeepEqual(role.Tags, tags) {
			tagKeys := []string{}
			for _, tag := range role.Tags {
				tagKeys = append(tagKeys, *tag.Key)
			}
			_, err := s.iamClient.UntagRole(ctx, &iam.UntagRoleInput{
				RoleName: aws.String(roleName),
				TagKeys:  tagKeys,
			})
			if err != nil {
				return fmt.Errorf("failed to untag IAM role %s: %w", roleName, err)
			}
			_, err = s.iamClient.TagRole(ctx, &iam.TagRoleInput{
				RoleName: aws.String(roleName),
				Tags:     tags,
			})
			if err != nil {
				return fmt.Errorf("failed to tag IAM role %s: %w", roleName, err)
			}
		}
	}

	var rolePolicy bytes.Buffer
	var data = RolePolicyData{
		AWSDomain:        awsDomain(s.cluster.Region),
		BucketName:       bucket.Spec.Name,
		ExtraBucketNames: bucket.Spec.AccessRole.ExtraBucketNames,
	}

	err = s.rolePolicy.Execute(&rolePolicy, data)
	if err != nil {
		return fmt.Errorf("failed to execute role policy template for role %s: %w", roleName, err)
	}

	_, err = s.iamClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(roleName),
		PolicyDocument: aws.String(rolePolicy.String()),
	})
	if err != nil {
		return fmt.Errorf("failed to put IAM role policy for role %s: %w", roleName, err)
	}
	return nil
}

func (s IAMAccessRoleServiceAdapter) DeleteRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	roleName := bucket.Spec.AccessRole.RoleName
	role, err := s.getRole(ctx, roleName)
	if err != nil {
		return fmt.Errorf("failed to get IAM role %s for deletion: %w", roleName, err)
	}

	if role == nil {
		s.logger.Info("IAM role does not exist, skipping deletion")
		return nil
	}
	// clean any attached policies, otherwise deletion of role will not work
	err = s.cleanAttachedPolicies(ctx, roleName)
	if err != nil {
		return fmt.Errorf("failed to clean attached policies for IAM role %s: %w", roleName, err)
	}

	_, err = s.iamClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:            aws.String(roleName),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NoSuchEntityException:
				s.logger.Info("no instance profile attached to role, skipping")
			default:
				return fmt.Errorf("failed to remove role %s from instance profile: %w", roleName, err)
			}
		}
	}

	_, err = s.iamClient.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NoSuchEntityException:
				s.logger.Info("no instance profile to delete, skipping")
			default:
				return fmt.Errorf("failed to delete instance profile %s: %w", roleName, err)
			}
		}
	}
	_, err = s.iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role %s: %w", roleName, err)
	}

	return nil
}

func (s *IAMAccessRoleServiceAdapter) cleanAttachedPolicies(ctx context.Context, roleName string) error {
	{
		o, err := s.iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			return fmt.Errorf("failed to list attached policies for IAM role %s: %w", roleName, err)
		} else {
			for _, p := range o.AttachedPolicies {
				policy := p
				s.logger.Info(fmt.Sprintf("detaching policy %s", *policy.PolicyName))

				_, err := s.iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
					PolicyArn: policy.PolicyArn,
					RoleName:  aws.String(roleName),
				})
				if err != nil {
					return fmt.Errorf("failed to detach policy %s from IAM role %s: %w", *policy.PolicyName, roleName, err)
				}

				s.logger.Info(fmt.Sprintf("detached policy %s", *policy.PolicyName))
			}
		}
	}

	// clean inline policies
	{
		o, err := s.iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			return fmt.Errorf("failed to list inline policies for IAM role %s: %w", roleName, err)
		}

		for _, p := range o.PolicyNames {
			policy := p
			s.logger.Info(fmt.Sprintf("deleting inline policy %s", policy))
			_, err := s.iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
				RoleName:   aws.String(roleName),
				PolicyName: aws.String(policy),
			})
			if err != nil {
				return fmt.Errorf("failed to delete inline policy %s from IAM role %s: %w", policy, roleName, err)
			}
			s.logger.Info(fmt.Sprintf("deleted inline policy %s", policy))
		}
	}

	s.logger.Info("cleaned attached and inline policies from IAM Role")
	return nil
}
