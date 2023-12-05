package aws

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
				"arn:aws:s3:::@BUCKET_NAME@",
				"arn:aws:s3:::@BUCKET_NAME@/*"
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

const trustIdentityPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Federated": "arn:aws:iam::@ACCOUNT_ID@:oidc-provider/irsa.@INSTALLATION@.@CLOUD_DOMAIN@"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"irsa.@INSTALLATION@.@CLOUD_DOMAIN@:sub": "system:serviceaccount:@SERVICE_ACCOUNT_NAMESPACE@:@SERVICE_ACCOUNT_NAME@"
				}
			}
		}
	]
}`

const bucketPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Sid": "EnforceSSLOnly",
			"Effect": "Deny",
			"Principal": "*",
			"Action": "s3:*",
			"Resource": [
				"arn:aws:s3:::@BUCKET_NAME@",
				"arn:aws:s3:::@BUCKET_NAME@/*"
			],
			"Condition": {
				"Bool": {
					"aws:SecureTransport": "false"
				}
			}
		}
	]
}`
