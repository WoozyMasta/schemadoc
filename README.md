# schemadoc

`schemadoc` is a JSON Schema documentation renderer.
It takes a schema as input and generates readable documentation
in Markdown (`list`, `table`) and HTML formats.
The main goal is practical: keep schema reference clear,
up to date, and easy to publish in repositories and CI pipelines.

Alongside documentation rendering, `schemadoc` includes
useful schema automation features.
It can generate JSON and YAML example configs from schema,
render YAML examples with schema-based comments,
merge multiple schema files into one resulting schema,
and generate schema from Go types through [CLI][].

The module is split by responsibility:

* package `schemadoc` renders docs and examples from JSON Schema
* package `merge` helper for merging schema fragments with deterministic rules
* package `modschema` reflects Go types into JSON Schema
* CLI `cmd/schemadoc` orchestrates these workflows automation pipeline

Real generated examples are available in [Generated Test Data][].
For example:

* [Schema example][]:
  merged JSON Schema that acts as source for the generated docs
* [Markdown example][]:
  one final Markdown reference document rendered from that schema
* [HTML preview][]: ([HTML raw][]):
  rendered HTML documentation from the schema
* [YAML example][]:
  YAML config example generated from the same schema

## CLI

The most common day-to-day scenario is simple:
take an existing schema and render documentation from it.
The command below produces list-style markdown.

```bash
schemadoc schema2doc --template list schema.json schema.list.md
```

For full CLI command behavior and flags, see [CLI Guide][CLI].

## Go Module

### Render Documentation From Schema

This snippet reads schema from file
and renders markdown using the built-in `list` template.

```go
doc, err := schemadoc.RenderFile("schema.json", schemadoc.Options{
    Title:        "Config Reference",
    TemplateName: "list",
    SourcePath:   "schema.json",
})
if err != nil {
    return err
}

fmt.Println(doc)
```

### Render Example Config From Schema

This call renders YAML example payload directly from schema.
Comments stay enabled,
so descriptions and defaults are visible in output.

```go
schemaBytes, err := os.ReadFile("schema.json")
if err != nil {
    return err
}

exampleYAML, err := schemadoc.GenerateExampleWithOptions(
    schemaBytes,
    schemadoc.ExampleModeAll,
    schemadoc.ExampleFormatYAML,
    schemadoc.ExampleOptions{
        YAMLIndent:             2,
        DisableExampleComments: false,
    },
)
if err != nil {
    return err
}

fmt.Println(string(exampleYAML))
```

### Merge Schema Fragments

This flow loads a base schema,
merges `$defs` from another file,
optionally prunes unreachable definitions,
and writes the result back.

```go
merged, err := merge.File(
    "app.schema.json",
    []merge.Action{
        {
            Type:          merge.NodeOpMergeDefs,
            SourcePath:    "shared.schema.json",
            SourcePointer: "/$defs",
            TargetPointer: "/$defs",
        },
    },
    merge.ApplyOptions{
        PruneUnreachableDefs: true,
    },
)
if err != nil {
    return err
}

encoded, err := merge.Encode(merged, merge.FormatJSON)
if err != nil {
    return err
}

if err := os.WriteFile("app.schema.json", encoded, 0o600); err != nil {
    return err
}
```

### Generate Schema From Go Types

If you only need basic reflection in application code,
`github.com/invopop/jsonschema` is often enough.

Use `modschema` when you need an end-to-end task-oriented flow:
resolve local or remote module targets, handle workspace replacements,
and generate schema through a temporary helper module
without hand-writing reflection glue code.

This call reflects one root type and returns schema bytes plus source label.
Use it when schema should be derived from code during build pipelines.

```go
schemaBytes, sourcePath, err := modschema.Generate(modschema.Options{
    Module:   ".",
    Package:  "github.com/acme/project/internal/config",
    Type:     "Config",
    KeyNamer: "none",
})
if err != nil {
    return err
}

fmt.Printf("source=%s bytes=%d\n", sourcePath, len(schemaBytes))
```

## Additional Notes

### Property Ordering With `x-order`

`x-order` is an optional numeric keyword on schema properties
that controls field order in generated output.
It is used in rendered docs and YAML examples.
Without it, `schemadoc` uses a stable default order.

### YAML Example Comments

YAML example generation supports optional schema-based comments.
When enabled, comments can include description, `default`, `example`,
and `enum` values.

Example schema fragment:

```yaml
properties:
  mode:
    type: string
    description: Execution mode.
    default: safe
    example: fast
    enum: [safe, fast]
```

Generated YAML example:

```yaml
# Execution mode.
# Default: safe
# Example: fast
# Allowed values: safe, fast
mode: safe
```

### Built-In And Custom Templates

CLI supports both built-in templates and custom templates.
Use `schemadoc template` to export built-in templates,
then pass your own template file via CLI flags
for project-specific rendering style.

### Template Helper Functions

Custom templates can use helper functions from renderer:

* `headingAnchor` for heading anchors
* `tableCell` for markdown-safe table cells
* `inlineHTML` for inline markdown-to-HTML conversion
* `attrHTML` for attribute-value HTML rendering
* `blockHTML` for simple markdown block rendering
* `tocHTML` for table-of-contents HTML rendering
* `renderAttrList` for attribute list rendering
* `jsonInline` for compact JSON value rendering
* `html` for generic HTML-escaping

<!-- links -->

[CLI]: cmd/schemadoc/doc/README.md
[Generated Test Data]: testdata/generated
[Schema example]: testdata/generated/app.schema.json
[Markdown example]: testdata/generated/app.doc.table.md
[HTML preview]: https://html-preview.github.io/?url=https://github.com/WoozyMasta/schemadoc/blob/master/testdata/generated/app.doc.html
[HTML raw]: testdata/generated/app.doc.html
[YAML example]: testdata/generated/app.config.yaml
