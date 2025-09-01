# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Update Kyverno PolicyExceptions to v2.

## [0.12.0] - 2025-06-26

### Changed

- Added hardcoded Trust Policy for `grafana-postgresql-recovery-test` service account in AWS IAM file.
- **Comprehensive error management improvements**: Migrated from `pkg/errors` to standard library errors and significantly improved error message consistency across the entire codebase:
  - Replaced all `github.com/pkg/errors` usage with standard `fmt.Errorf()` and `%w` verb for error wrapping
  - Removed stack trace logging and merged `logger.Error` + `return err` patterns into single `return fmt.Errorf()` calls
  - Standardized error patterns for consistent debugging experience across all components

## [0.11.0] - 2025-05-12

### Fixed

- Azure: read ClusterIdentity's namespace from Cluster resource

## [0.10.4] - 2025-04-23

### Fixed

- Fix golangci-lint v2 problems.

## [0.10.3] - 2025-03-13

### Changed

- Stop caching helm secrets in the operator to reduce resource usage.
- Use smaller dockerfile to reduce build time as ABS already generates the go binary.

## [0.10.2] - 2025-02-06

### Fixed

- Fix xml output for us-east-1 region as the bucket creation config needs to be empty.

## [0.10.1] - 2025-02-06

### Fixed

- Fix bucket creation in us-east-1.

## [0.10.0] - 2025-01-07

### Changed

- Secure Azure Storage Account by making them private and accessible through an Azure Private Endpoint. This also requires the creation of a private DNS zone and A record.
- Update Kyverno PolicyException to v2beta1.

### Removed

- Remove PSP.

## [0.9.0] - 2024-10-03

### Added

- Add doc and unit tests using github copilot.

### Fixed

- Disable logger development mode to avoid panicking, use zap as logger
- Fix `irsa domain` in China after we migrated the irsa domain to `oidc-pod-identity-v3`.

## [0.8.0] - 2024-07-17

### Added

- ReclaimPolicy added in the Bucket CR to manage the data clean up (retain or delete).
- Add a finalizer on the Azure secret to prevent its deletion.
- Empty all the objects in the S3 bucket in case of bucket deletion.

## [0.7.0] - 2024-06-18

### Changed

- Change azure storage account secret name by using the bucket name instead of the storage account name to not be bothered by azure storage account name limitations (up to 24 characters) which truncates secret name for long bucket names like `giantswarm-glippy-mimir-ruler` which becomes `giantswarmglippymimirrul`. As this rule is unpredictable (depends on the installation name), it is better to fix the name of the secret.

## [0.6.1] - 2024-06-17

### Fixed

- Fix object-storage-operator aws templating by using the root scope when possible.

## [0.6.0] - 2024-06-17

### Changed

- Add support for the region of China.

## [0.5.5] - 2024-05-13

### Fixed

- Add basic tag key sanitization for azure bucket tags as they need to match c# identifiers.

## [0.5.4] - 2024-04-08

### Fixed

- Fix KyvernoPolicyException to apply when podSecurityStandard is enabled.

## [0.5.3] - 2024-03-07

### Fixed

- Fix `ConfigureRole` method while untagging bucket (removing empty value in array creation).

## [0.5.2] - 2024-03-07

### Changed

- Set metrics port in deployment and use it in PodMonitor spec.

## [0.5.1] - 2024-03-06

### Changed

- Update deprecated `targetPort` to `port` in PodMonitor.

## [0.5.0] - 2024-02-15

### Changed

- Change rendering of bucket policies to use template/text instead of a string to be able to add extra bucket access (needed for the mimir ruler)

## [0.4.3] - 2024-01-11

### Fixed

- Fix metrics and probes ports.

## [0.4.2] - 2024-01-11

### Fixed

- Fix listenPort to avoid 8081 already used by `azure-private-endpoint-operator`.

## [0.4.1] - 2024-01-10

### Fixed

- Fix PolicyException and PSP.

## [0.4.0] - 2023-12-06

### Added

- Implement creation of Azure Storage Containers on CAPZ management clusters.

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.
- Abstract managementcluster (refactoring).
- Enforce encryption in transit for s3 Buckets.

## [0.3.0] - 2023-11-22

### Added

- Add installation additional tags to cloud resources.

## [0.2.1] - 2023-11-13

### Fixed

- Fix issues in networkpolicy.

## [0.2.0] - 2023-11-09

### Added

- Add bucket access role creation in the operator.

## [0.1.0] - 2023-10-31

### Added

- Implement creation of S3 buckets on CAPA management clusters.

[Unreleased]: https://github.com/giantswarm/object-storage-operator/compare/v0.12.0...HEAD
[0.12.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.10.4...v0.11.0
[0.10.4]: https://github.com/giantswarm/object-storage-operator/compare/v0.10.3...v0.10.4
[0.10.3]: https://github.com/giantswarm/object-storage-operator/compare/v0.10.2...v0.10.3
[0.10.2]: https://github.com/giantswarm/object-storage-operator/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/giantswarm/object-storage-operator/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/giantswarm/object-storage-operator/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.5...v0.6.0
[0.5.5]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.3...v0.5.4
[0.5.3]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.4.3...v0.5.0
[0.4.3]: https://github.com/giantswarm/object-storage-operator/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/giantswarm/object-storage-operator/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/giantswarm/object-storage-operator/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/giantswarm/object-storage-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/object-storage-operator/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/giantswarm/object-storage-operator/releases/tag/v0.1.0
