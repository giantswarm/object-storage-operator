# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Fix `port` type from integer to string.

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

[Unreleased]: https://github.com/giantswarm/object-storage-operator/compare/v0.5.1...HEAD
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
