# Changelog

We document all notable changes to this project in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2021-03-22

### Changed

* The location of the `validation` and `obj` packages has been moved to better represent the specific APIs they work with.

### Build

* Images are now uploaded with the tagged version instead of "latest".

## [0.2.0] - 2021-03-18

### Added

* The name of the PVC to be checked out is now configurable.
* The maximum backoff duration for controller reconcilers is now configurable with a default of 1 minute.

### Changed

* A checkout's spec is now immutable when it has selected a volume to use from a pool.

### Fixed

* Copy the `AccessModes` requested in a checkout to the checkout's volume.
* A checkout always acquires a new volume from the pool if the claim of the volume it references is deleted.

## [0.1.4] - 2021-03-16

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

[Unreleased]: https://github.com/puppetlabs/pvpool/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/puppetlabs/pvpool/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/puppetlabs/pvpool/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/puppetlabs/pvpool/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/puppetlabs/pvpool/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/puppetlabs/pvpool/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/puppetlabs/pvpool/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/puppetlabs/pvpool/compare/5aad04bb4bcc20306103a240b676ea310d6732af...v0.1.0
