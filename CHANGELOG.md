# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.2.0][] - 2026-02-20

### Added

* Example document generation API in package: `GenerateExample(...)`,
  `GenerateExampleJSON(...)`, `GenerateExampleYAML(...)`.
* CLI commands `schema2json` and `schema2yaml` for direct example output.
* YAML example comments from schema metadata `title` and `description`.

### Changed

* `schema2md` and `mod2md` can now embed generated embedded example payload
  at the end of markdown.

[0.2.0]: https://github.com/WoozyMasta/paa/compare/v0.1.0...v0.2.0

## [0.1.0][] - 2026-02-19

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/schemadoc/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
