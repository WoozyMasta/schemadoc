// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package schemadoc generates reference documentation
and example configurations from JSON Schema.
It helps keep configuration references, sample files,
and published documentation consistent with the schemas maintained in a project.

[Render] and [RenderFile] produce Markdown or HTML documentation
describing schema definitions, properties, constraints, defaults, and relationships.
Built-in templates provide list, table, and HTML layouts;
[Options] also accepts custom Go templates, source metadata,
and embedded configuration examples.

[GenerateExample] generates JSON or YAML payloads
containing either all properties or only required properties.
Values come from schema examples, defaults, and type-based fallbacks.
[GenerateExampleWithOptions] controls formatting and schema-derived comments in YAML output.

The module includes the merge subpackage for composing schema fragments
and the modschema subpackage for generating schemas from Go types.
These libraries can be used independently of the CLI.

The schemadoc CLI combines Go reflection, schema merging, documentation,
and example generation into YAML-configured build pipelines.
Its check mode compares generated artifacts with existing files without rewriting them,
so the same pipeline can generate files locally and verify them in CI.

Rendering resolves local JSON Pointer references (#/...) without network access.
Remote references and plain-name anchors are unsupported.
Output is deterministic for the same schema, template, and options.
*/
package schemadoc
