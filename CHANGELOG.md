<!-- markdownlint-disable MD024 -->
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

## Unreleased

### Changed

* Local module targets with a `/vN` major-version suffix
  now use a Go-compatible placeholder requirement version
  in the temporary helper module.

## [0.7.0][] - 2026-08-29

### Added

* `--show-internal-keywords` and `show_internal_keywords`
  to include renderer-specific schema keywords in generated documentation.

### Changed

* `x-order` is now applied consistently to YAML examples,
  definition headings, and Markdown/HTML table of contents entries.
* Internal schema keywords such as `x-order`
  are hidden from documentation by default.
* File-backed YAML examples now include a `yaml-language-server` schema hint;
  GitHub-backed schemas use their Raw URL when available.

[0.7.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.6.0...v0.7.0

## [0.6.0][] - 2026-08-14

### Added

* `--footer` for `schema2doc` and `mod2doc`,
  plus `schema2doc.footer` in build configuration,
  to include schemadoc version metadata.

### Changed

* Improved Markdown documentation generation for Go doc comments,
  including lists and preformatted blocks.
* Generated documents no longer include schemadoc version metadata by default.
* Generated CLI and schema documentation now use a canonical program name
  and include generator metadata.

[0.6.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.5.3...v0.6.0

## [0.5.3][] - 2026-05-02

### Changed

* Updated `github.com/woozymasta/flags`

[0.5.3]: https://github.com/WoozyMasta/schemadoc/compare/v0.5.2...v0.5.3

## [0.5.2][] - 2026-05-02

### Added

* New CLI flag `--jsonschema-version` for `mod2schema` and `mod2doc` to
  override helper dependency version for `github.com/invopop/jsonschema`.

### Changed

* Replaced `github.com/jessevdk/go-flags` with `github.com/woozymasta/flags`
* `modschema` helper now pins `github.com/invopop/jsonschema`
  to `v0.14.0` by default.
* Added default `go >= 1.24` precheck for `modschema` helper
  when `--jsonschema-version` is not provided.

### Fixed

* `modschema` Go version precheck now probes local toolchain in isolated
  context and no longer triggers toolchain auto-downloads from unrelated
  project `go.mod` files.

[0.5.2]: https://github.com/WoozyMasta/schemadoc/compare/v0.5.1...v0.5.2

## [0.5.1][] - 2026-03-27

### Changed

* `merge` command now uses `go.yaml.in/yaml/v3` directly for YAML processing.
* Removed dependency on `github.com/invopop/yaml` and related transitives.

[0.5.1]: https://github.com/WoozyMasta/schemadoc/compare/v0.5.0...v0.5.1

## [0.5.0][] - 2026-03-26

### Added

* YAML-based `build` orchestration pipeline for multi-stage generation:
  schema reflection, merge, docs rendering, and JSON/YAML example generation.
* Separated packages `modschema` for schema generation from Go types
* Package `merge` for deterministic schema composition.
* Support for hiding non-standard schema keywords in rendered docs.
* Validation for unsupported reflection targets,
  including `package main` in module reflection.

### Changed

* CLI refactored into command-scoped modules with cleaner execution boundaries.
* Renamed commands `schema2md` and `mod2md` to `schema2doc` and `mod2doc`.

### Fixed

* TOC hierarchy rendering in HTML output.
* Rendering of constraint values (for example regex `pattern`) in docs.

[0.5.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.4.0...v0.5.0

## [0.4.0][] - 2026-03-21

### Added

* `schema2md` and `mod2md` now support rendering HTML output with
  `--template html`.
* `schema2md` and `mod2md` now support adding custom top-level description
  to rendered output with `--description`.

### Changed

* Markdown built-in templates (`list` and `table`) now render optional
  top-level description and optional generation footer.

### Fixed

* `mod2schema` and `mod2md` now correctly resolve local module replacement
  chains from `go.mod replace` and module graph (`go list -m -json all`),
  including local workspace (`go.work`) scenarios.

[0.4.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.3.2...v0.4.0

## [0.3.2][] - 2026-03-07

### Fixed

* Fixed broken example markdown file links in `schema2md` output.

[0.3.2]: https://github.com/WoozyMasta/schemadoc/compare/v0.3.1...v0.3.2

## [0.3.1][] - 2026-03-07

### Added

* Comments about automatic generation to Markdown template files:
  `<!-- Automatically generated file, do not modify! -->`
* Nested TOC entries rendering in markdown docs based on
  definition reference graph.
* Per-segment path links in `Path` and `Paths` output when target
  anchors can be resolved.
* `mod2schema` helper `TypeNamer` to avoid `$defs` name collisions
  for same-named types from different packages.
* Markdown header now includes GitHub source links:
  `Source file` (browser URL) and `Source URL` (`Raw schema URL`) when schema
  ID and source path allow URL resolution.
* Embedded YAML example now includes
  `# yaml-language-server: $schema=...` when raw schema URL is available.

### Changed

* `$ref` attributes are now rendered as readable markdown links to local
  definition sections when possible, while preserving raw pointer value.
* Table cells are now normalized to single line.
* Header metadata labels are now more user-friendly.

[0.3.1]: https://github.com/WoozyMasta/schemadoc/compare/v0.3.0...v0.3.1

## [0.3.0][] - 2026-03-06

### Added

* `mod2schema` and `mod2md` now support `--key-namer` with
  `none|snake|kebab|lower` strategies for fields without explicit `json` tags.

### Changed

* `render_attributes` now expands nested schema-like keywords into readable
  attribute rows (for example `Items type`, `Items enum`, `Items examples`)
  instead of one dense inline line.
* String values in inline render output are shown without JSON quotes for
  better readability.
* Example generation for arrays now uses all `items.examples` values when they
  are present.

[0.3.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.2.0...v0.3.0

## [0.2.0][] - 2026-02-20

### Added

* Example document generation API in package: `GenerateExample(...)`,
  `GenerateExampleJSON(...)`, `GenerateExampleYAML(...)`.
* CLI commands `schema2json` and `schema2yaml` for direct example output.
* YAML example comments from schema metadata `title` and `description`.

### Changed

* `schema2md` and `mod2md` can now embed generated embedded example payload
  at the end of markdown.

[0.2.0]: https://github.com/WoozyMasta/schemadoc/compare/v0.1.0...v0.2.0

## [0.1.0][] - 2026-02-19

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/schemadoc/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
