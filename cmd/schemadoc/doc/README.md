# schemadoc CLI

`schemadoc` keeps schema references and example configurations
in sync with JSON Schema or Go types.
It produces Markdown/HTML documentation and JSON/YAML examples,
either as individual commands or as one repeatable build pipeline.

For command syntax and flags, see the generated [CLI reference][].
For build fields, see the generated [config reference][].

## Basic workflow

For an existing schema, a single command is enough:

```bash
schemadoc schema2doc schema.json reference.md
```

For a project with several generated artifacts,
keep their inputs and outputs in `schemadoc.build.yaml`.
For example, generate a schema from a Go type,
then render documentation and a YAML example from it:

```yaml
schema: schemas/config.schema.json
mod2schema:
  module: .
  type: Config
  comments: plain
schema2doc:
  template: table
  output: docs/config.md
schema2yaml:
  output: config.example.yaml
```

Run from the project directory:

```bash
schemadoc build
schemadoc build --check
```

The first command generates the files.
The second compares expected content with existing files
and fails on the first difference or missing file,
without rewriting artifacts. Use the same config locally and in CI.
Relative paths resolve from the current working directory.

## Pipeline model

Each YAML document is one job; separate jobs with `---`. Jobs run in file order.
Within a job, declared stages run in fixed order, regardless of YAML key order:

`mod2schema -> mod2doc -> merge -> schema2json -> schema2doc -> schema2yaml`.

`schema` is the job's working file:
`mod2schema` writes it, `merge` modifies it, and `schema2*` stages consume it.
Omit stages you do not need; an existing schema can be the starting point.

`mod2doc` instead renders a transient schema directly
from the sibling `mod2schema` Go target, before merge.
Use it to preserve Go documentation independently of the persisted schema.
`schema2doc` reflects the working schema,
including patches and imported definitions.
Go comments remain raw by default;
`comments: plain` normalizes them for portable schema descriptions.

One standalone `defaults:` document
can supply shared settings anywhere in the stream.
Jobs override its values: maps merge recursively, lists replace,
and `null` removes an inherited field or stage.
Explicit `false`, `0`, and empty strings
remain overrides subject to field validation.
A second defaults document is an error.
Defaults do not count towards `--index`;
`--check` applies to every selected job regardless of its `check` value.

## Important constraints

* Go reflection requires `go` in `PATH` and an importable target package;
  `package main` is unsupported.
  Use a local module directory containing `go.mod`
  or a remote module reference with an explicit version.
* Reflect only trusted modules:
  target and transitive package initialization executes with your permissions,
  without a sandbox. Module checksums verify integrity, not trust.
  `@latest` is non-deterministic and rejected in check mode.
* `mod2doc` comments default to `raw`, independently of `mod2schema.comments`.
  If external type names collide, use `mod2schema.type_namer: full`.
* The editor schema currently describes complete jobs;
  it does not yet support defaults documents or partial jobs.
  The CLI validates defaults and checks required fields after merging,
  before executing jobs.

<!-- links -->

[CLI reference]: CLI.md
[config reference]: config.table.md
