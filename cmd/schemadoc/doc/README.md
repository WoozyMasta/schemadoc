# schemadoc CLI

`schemadoc` CLI is a command-line interface for JSON Schema workflows.
It can be used in local work and in CI pipelines.
The tool runs schema generation, merge, documentation rendering,
and example rendering in a reproducible way.

The CLI provides these commands:

* `schema2doc` renders docs from JSON Schema using selected template.
  Built-in templates are `list`, `table`, and `html`.
* `schema2json` renders a JSON example config from schema,
  filling keys from schema structure and example/default values.
* `schema2yaml` renders a YAML example config from schema,
  with optional comments from description/default/example/enum.
* `mod2schema` reflects a Go type and writes a JSON or YAML schema file.
* `mod2doc` runs `mod2schema` + `schema2doc` in one command.
* `merge` applies schema imports and patches to build one combined schema.
* `build` executes multi-stage pipeline from YAML config documents.
* `template` prints built-in documentation templates for customization.
* `config` prints an example build config generated from config schema.
* `help` prints help for commands and flags.
* `completion` prints shell completion script (`bash`, `zsh`, `pwsh`).
* `docs` prints generated CLI docs (`man`, `md`, `html`).
* `version` and `--version` print CLI build/version metadata.

Use `schemadoc -h` and `schemadoc <command> -h`
for exact arguments and flags.
Use `schemadoc --version` for short version output.

## Quick Usage

Render docs from schema:

```bash
schemadoc schema2doc schema.json schema.list.md
schemadoc schema2doc --template table schema.json schema.table.md
schemadoc schema2doc --template html schema.json schema.html
```

Generate config examples from schema:

```bash
schemadoc schema2json schema.json config.example.json
schemadoc schema2yaml schema.json config.example.yaml
schemadoc schema2yaml --disable-example-comments schema.json clean.yaml
```

Reflect schema from Go type:

```bash
schemadoc mod2schema --type Config . schema.json
schemadoc mod2schema \
  --package github.com/acme/project/internal/config \
  --type Config github.com/acme/project@v1.2.3 schema.json
```

Generate docs from Go type:

```bash
schemadoc mod2doc --type Config . config.list.md
schemadoc mod2doc --template table --type Config . config.table.md
```

Merge schemas:

```bash
schemadoc merge base.schema.json out.schema.json \
  --merge-defs '/$defs=shared.schema.json#/$defs'
```

## Build Pipeline

Command `build` executes an automated pipeline from YAML config.
Each YAML document describes one run with ordered stages and outputs.
Config format is documented in [config.table.md][],
and example config is in [config.example.yaml][].

Stages run in fixed order:  
`mod2schema -> merge -> schema2json -> schema2doc -> schema2yaml`.  
Execution stops on the first error.

Behavior:

* config path is optional; default is `./schemadoc.build.yaml`
* `--index 0` runs all YAML documents in file order
* `--index 1..N` runs one selected YAML document
* `check: true` in config enables validation mode (no rewrites)

Examples:

```bash
schemadoc build
schemadoc build schemadoc.build.yaml
schemadoc build schemadoc.build.yaml --index 2
```

## mod2schema/mod2doc Requirements

`mod2schema` and `mod2doc` require installed Go toolchain
(`go` must be available in `PATH`).

`module` argument supports:

* local module directory (must contain `go.mod`)
* remote module reference with explicit version
  (`github.com/acme/project@v1.2.3`)

Reflection target package must be importable.
`package main` is not supported as target for reflection.

## Config Reference Artifacts

Generated config docs and examples:

* [config.html][] ([preview][])
* [config.example.json][]
* [config.schema.json][]
* [config.list.md][]
* [config.table.md][]
* [config.example.yaml][]

<!-- links -->

[config.html]: config.html
[preview]: https://html-preview.github.io/?url=https://github.com/WoozyMasta/schemadoc/blob/master/cmd/schemadoc/doc/config.html
[config.example.json]: config.example.json
[config.schema.json]: config.schema.json
[config.list.md]: config.list.md
[config.table.md]: config.table.md
[config.example.yaml]: config.example.yaml
