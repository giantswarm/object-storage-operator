package aws

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
)

type IAMAccessRoleServiceAdapter struct {
	iamClient   *iam.Client
	logger      logr.Logger
	accountId   string
	baseDomain  string
	clusterName string
}

func NewIamService(iamClient *iam.Client, logger logr.Logger, accountId string, cluster cluster.Cluster) IAMAccessRoleServiceAdapter {
	return IAMAccessRoleServiceAdapter{
		iamClient:   iamClient,
		logger:      logger,
		accountId:   accountId,
		baseDomain:  cluster.GetBaseDomain(),
		clusterName: cluster.GetName(),
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
				s.logger.Error(err, "Failed to fetch IAM Role")
				return nil, errors.WithStack(err)
			}
		}
		return nil, errors.WithStack(err)
	}
	return output.Role, nil
}

func (s IAMAccessRoleServiceAdapter) ConfigureRole(ctx context.Context, bucket *v1alpha1.Bucket, additionalTags map[string]string) error {
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
	for k, v := range additionalTags {
		// We use this to avoid pointer issues in range loops.
		key := k
		value := v
		if key != "" && value != "" {
			tags = append(tags, types.Tag{Key: &key, Value: &value})
		}
	}

	trustPolicy := s.templateTrustPolicy(bucket)

	if role == nil {
		_, err := s.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
			RoleName:                 aws.String(roleName),
			AssumeRolePolicyDocument: aws.String(trustPolicy),
			Description:              aws.String("Role for Giant Swarm managed Loki"),
			Tags:                     tags,
		})
		if err != nil {
			return errors.WithStack(err)
		}
		s.logger.Info("IAM Role created")
	} else {
		_, err = s.iamClient.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
			RoleName:       aws.String(roleName),
			PolicyDocument: aws.String(trustPolicy),
		})
		if err != nil {
			return errors.WithStack(err)
		}

		// Update tags (need to untag with existing keys then retag)
		if !reflect.DeepEqual(role.Tags, tags) {
			tagKeys := make([]string, len(role.Tags))
			for _, tag := range role.Tags {
				tagKeys = append(tagKeys, *tag.Key)
			}
			_, err := s.iamClient.UntagRole(ctx, &iam.UntagRoleInput{
				RoleName: aws.String(roleName),
				TagKeys:  tagKeys,
			})
			if err != nil {
				return errors.WithStack(err)
			}
			_, err = s.iamClient.TagRole(ctx, &iam.TagRoleInput{
				RoleName: aws.String(roleName),
				Tags:     tags,
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	_, err = s.iamClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(roleName),
		PolicyDocument: aws.String(templateRolePolicy(bucket)),
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s IAMAccessRoleServiceAdapter) DeleteRole(ctx context.Context, bucket *v1alpha1.Bucket) error {
	roleName := bucket.Spec.AccessRole.RoleName
	role, err := s.getRole(ctx, roleName)
	if err != nil {
		return err
	}

	if role == nil {
		s.logger.Info("IAM role does not exist, skipping deletion")
		return nil
	}
	// clean any attached policies, otherwise deletion of role will not work
	err = s.cleanAttachedPolicies(ctx, roleName)
	if err != nil {
		return err
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
				s.logger.Error(err, "failed to remove role from instance profile")
				return errors.WithStack(err)
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
				s.logger.Error(err, "failed to delete instance profile")
				return errors.WithStack(err)
			}
		}
	}
	_, err = s.iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *IAMAccessRoleServiceAdapter) cleanAttachedPolicies(ctx context.Context, roleName string) error {
	{
		o, err := s.iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			return err
		} else {
			for _, p := range o.AttachedPolicies {
				policy := p
				s.logger.Info(fmt.Sprintf("detaching policy %s", *policy.PolicyName))

				_, err := s.iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
					PolicyArn: policy.PolicyArn,
					RoleName:  aws.String(roleName),
				})
				if err != nil {
					s.logger.Error(err, fmt.Sprintf("failed to detach policy %s", *policy.PolicyName))
					return err
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
			s.logger.Error(err, "failed to list inline policies")
			return err
		}

		for _, p := range o.PolicyNames {
			policy := p
			s.logger.Info(fmt.Sprintf("deleting inline policy %s", policy))
			_, err := s.iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
				RoleName:   aws.String(roleName),
				PolicyName: aws.String(policy),
			})
			if err != nil {
				s.logger.Error(err, fmt.Sprintf("failed to delete inline policy %s", policy))
				return err
			}
			s.logger.Info(fmt.Sprintf("deleted inline policy %s", policy))
		}
	}

	s.logger.Info("cleaned attached and inline policies from IAM Role")
	return nil
}

func (s IAMAccessRoleServiceAdapter) templateTrustPolicy(bucket *v1alpha1.Bucket) string {
	policy := strings.ReplaceAll(trustIdentityPolicy, "@CLOUD_DOMAIN@", s.baseDomain)
	policy = strings.ReplaceAll(policy, "@INSTALLATION@", s.clusterName)
	policy = strings.ReplaceAll(policy, "@ACCOUNT_ID@", s.accountId)
	policy = strings.ReplaceAll(policy, "@SERVICE_ACCOUNT_NAMESPACE@", bucket.Spec.AccessRole.ServiceAccountNamespace)
	policy = strings.ReplaceAll(policy, "@SERVICE_ACCOUNT_NAME@", bucket.Spec.AccessRole.ServiceAccountName)
	return policy
}

func templateRolePolicy(bucket *v1alpha1.Bucket) string {
	return strings.ReplaceAll(rolePolicy, "@BUCKET_NAME@", bucket.Spec.Name)
}
