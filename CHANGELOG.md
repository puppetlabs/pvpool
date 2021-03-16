# Changelog

We document all notable changes to this project in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

* Remove arbitrary restriction on `AccessModes` for pool claims.

## [0.1.3] - 2021-03-15

### Fixed

* Correctly copy the StorageClass of a persistent volume to a corresponding checkout's claim.

## [0.1.2] - 2021-03-05

### Changed

* Support Kubernetes 1.16 by accepting v1beta1 AdmissionReview objects.

## [0.1.1] - 2021-02-24

### Build

* Release an archive file for Kustomize to use as a base.

## [0.1.0] - 2021-02-24

### Added

* Initial release.

[Unreleased]: https://github.com/puppetlabs/pvpool/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/puppetlabs/pvpool/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/puppetlabs/pvpool/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/puppetlabs/pvpool/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/puppetlabs/pvpool/compare/5aad04bb4bcc20306103a240b676ea310d6732af...v0.1.0
