# Controllers

## Bucket controller

This controller reconciles `Buckets`. It creates a cloud provider bucket in the Management Cluster Region.
It currently only supports AWS S3 on CAPA management clusters and Azure Storage Container on CAPZ management clusters.

### CAPZ resources

To handle an object storage on Azure, we need to create:

- a storage account
- a storage container

We choose to create a unique relation Storage Account - Storage Container to have a proper clean up when a bucket is deleted. This way, there won't have orphan storage account on Azure.

We add a lifecyle management rule on the storage account to clean old data (`bucket.spec.expirationPolicy.days`)

When the object storage is created, we retrieve the Access Key and create a secret in the bucket namespace containing the name of the storage account and the access key. This secret is necessary for the application desiring to use this object storage.

# Testing

You can run all tests with

```
make test-all
```

## Unit tests

You can run only the unit tests with

```
make test-unit
```

## Integration tests

You can run only the integration tests with

```
make test-integration
```

This project uses [`LocalStack`](https://github.com/localstack/localstack) for integration tests. Remember that you can use the `aws` cli targetting the local [`LocalStack`](https://github.com/localstack/localstack) environment with
```
$ aws --endpoint=http://localhost:4566 s3 list-buckets
```
