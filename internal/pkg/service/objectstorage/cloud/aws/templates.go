package aws

import (
	"strings"
)

type RolePolicyData struct {
	AWSDomain        string
	BucketName       string
	ExtraBucketNames []string
}

func awsDomain(region string) string {
	domain := "aws"

	if isChinaRegion(region) {
		domain = "aws-cn"
	}

	return domain
}

func isChinaRegion(region string) bool {
	return strings.Contains(region, "cn-")
}

const rolePolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:ListBucket",
				"s3:PutObject",
				"s3:GetObject",
				"s3:DeleteObject"
			],
			"Resource": [
				{{ range .ExtraBucketNames }}
				"arn:{{ $.AWSDomain }}:s3:::{{ . }}",
				"arn:{{ $.AWSDomain }}:s3:::{{ . }}/*",
				{{ end }}
				"arn:{{ $.AWSDomain }}:s3:::{{ $.BucketName }}",
				"arn:{{ $.AWSDomain }}:s3:::{{ $.BucketName }}/*"
			]
		},
		{
			"Effect": "Allow",
			"Action": [
				"s3:GetAccessPoint",
				"s3:GetAccountPublicAccessBlock",
				"s3:ListAccessPoints"
			],
			"Resource": "*"
		}
	]
}`

type TrustIdentityPolicyData struct {
	AccountId               string
	AWSDomain               string
	CloudFrontDomain        string
	ServiceAccountName      string
	ServiceAccountNamespace string
}

const trustIdentityPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Federated": "arn:{{ $.AWSDomain }}:iam::{{ $.AccountId }}:oidc-provider/{{ $.CloudFrontDomain }}"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"{{ $.CloudFrontDomain }}:sub": "system:serviceaccount:{{ $.ServiceAccountNamespace }}:{{ $.ServiceAccountName }}"
				}
			}
		}
	]
}`

type GrafanaTrustIdentityPolicyData struct {
	AccountId               string
	AWSDomain               string
	CloudFrontDomain        string
	ServiceAccountName      string
	ServiceAccountNamespace string
}

const GrafanaTrustIdentityPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Federated": "arn:{{ $.AWSDomain }}:iam::{{ $.AccountId }}:oidc-provider/{{ $.CloudFrontDomain }}"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"{{ $.CloudFrontDomain }}:sub": "system:serviceaccount:{{ $.ServiceAccountNamespace }}:{{ $.ServiceAccountName }}"
				}
			}
		},
		{
			"Effect": "Allow",
			"Principal": {
				"Federated": "arn:{{ $.AWSDomain }}:iam::{{ $.AccountId }}:oidc-provider/{{ $.CloudFrontDomain }}"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"{{ $.CloudFrontDomain }}:sub": "system:serviceaccount:monitoring:grafana-postgresql-recovery-test"
				}
			}
		}
	]
}`

type BucketPolicyData struct {
	AWSDomain  string
	BucketName string
}

const bucketPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "EnforceSSLOnly",
			"Effect": "Deny",
			"Principal": "*",
			"Action": "s3:*",
			"Resource": [
				"arn:{{ $.AWSDomain }}:s3:::{{ $.BucketName }}",
				"arn:{{ $.AWSDomain }}:s3:::{{ $.BucketName }}/*"
			],
			"Condition": {
				"Bool": {
					"aws:SecureTransport": "false"
				}
			}
		}
	]
}`
