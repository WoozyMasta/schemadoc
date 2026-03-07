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
