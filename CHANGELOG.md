# Changelog
All notable changes to this project will be documented in this file.

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v0.2.0
### Added
- Add `address` and `apps` packages with additional functionality ([#16](https://github.com/algorand/avm-abi/pull/16))

## v0.1.1
### Added
- Expanded documentation for `Encode` and `Decode` functions ([#11](https://github.com/algorand/avm-abi/pull/11))
### Fixed
- Allow zero-length static arrays ([#14](https://github.com/algorand/avm-abi/pull/14))
- Fix type parsing bug for tuples containing static arrays of tuples ([#13](https://github.com/algorand/avm-abi/pull/13))

## v0.1.0
### Added
- The `abi.Type` struct and `abi.TypeOf` function, which support basic ARC-4
  data types.
