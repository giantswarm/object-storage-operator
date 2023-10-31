# Controllers

## Bucket controller

This controller reconciles `Buckets`. It creates a cloud provider bucket in the Management Cluster Region.
It currently only supports AWS S3 on CAPA management clusters.

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
