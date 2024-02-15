package aws

type RolePolicyData struct {
	BucketName       string
	ExtraBucketNames []string
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
				"arn:aws:s3:::{{ . }}",
				"arn:aws:s3:::{{ . }}/*",
				{{ end }}
				"arn:aws:s3:::{{ .BucketName }}",
				"arn:aws:s3:::{{ .BucketName }}/*"
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
	CloudDomain             string
	Installation            string
	ServiceAccountName      string
	ServiceAccountNamespace string
}

const trustIdentityPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Federated": "arn:aws:iam::{{ .AccountId }}:oidc-provider/irsa.{{ .Installation }}.{{ .CloudDomain }}"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"irsa.{{ .Installation }}.{{ .CloudDomain }}:sub": "system:serviceaccount:{{ .ServiceAccountNamespace }}:{{ .ServiceAccountName }}"
				}
			}
		}
	]
}`

type BucketPolicyData struct {
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
				"arn:aws:s3:::{{ .BucketName }}",
				"arn:aws:s3:::{{ .BucketName }}/*"
			],
			"Condition": {
				"Bool": {
					"aws:SecureTransport": "false"
				}
			}
		}
	]
}`
