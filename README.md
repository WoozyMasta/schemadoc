# schemadoc

`schemadoc` generates reference documentation
and example configurations from JSON Schema or Go types.
Use it to keep checked-in schemas, configuration samples,
and published references consistent with their source.

* Render Markdown (`list`, `table`) or HTML using built-in or custom templates.
* Generate JSON/YAML examples with all or only required properties;
  YAML can include schema-derived comments.
* Generate schemas from Go types and compose schemas from reusable fragments.
* Run these steps from one build config, then check the same artifacts in CI.

## Install

Install with Go 1.25 or newer:

```bash
go install github.com/woozymasta/schemadoc/cmd/schemadoc@latest
```

From this checkout, use `go install ./cmd/schemadoc`.
Go reflection also requires `go` in `PATH` at runtime.

## Quick start

Render an existing schema:

```bash
schemadoc schema2doc --template table schema.json reference.md
```

For project automation, describe inputs and outputs in `schemadoc.build.yaml`:

```bash
schemadoc build
schemadoc build --check
```

`--check` compares generated content with existing files without rewriting them.
The [CLI guide][] explains the pipeline, global defaults, and source selection.
See the generated [command reference][]
and [config reference][] for exact options.

## Generated output

One example schema is used to generate these artifacts:

* [JSON Schema][]
* [Markdown reference][]
* [HTML reference][] ([preview][])
* [YAML configuration][]

Property keyword `x-order` controls ordering in documentation and YAML examples.
For project-specific layouts, see the [custom template guide][].

## Go libraries

The libraries are usable independently of the CLI.
Executable examples cover these entry points:

* [schemadoc][]: render documentation and example payloads.
* [merge][]: compose schema fragments in memory or from files.
* [modschema][]: generate schemas from Go modules,
  reuse prepared helpers, and cancel requests.

## Constraints

* Rendering resolves local JSON Pointer references (`#/...`)
  without network access.
  Remote references and plain-name anchors are unsupported.
* Reflect only trusted Go modules: package initialization
  executes with the caller's permissions, without a sandbox.
  Targets must be importable packages; `package main` is unsupported.

<!-- links -->

[CLI guide]: cmd/schemadoc/doc/README.md
[command reference]: cmd/schemadoc/doc/CLI.md
[config reference]: cmd/schemadoc/doc/config.table.md
[JSON Schema]: testdata/generated/app.schema.json
[Markdown reference]: testdata/generated/app.doc.table.md
[HTML reference]: testdata/generated/app.doc.html
[preview]: https://html-preview.github.io/?url=https://github.com/WoozyMasta/schemadoc/blob/master/testdata/generated/app.doc.html
[YAML configuration]: testdata/generated/app.config.yaml
[custom template guide]: templates/README.md
[schemadoc]: example_usage_test.go
[merge]: merge/example_test.go
[modschema]: modschema/example_test.go
