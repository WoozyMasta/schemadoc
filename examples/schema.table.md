<!-- Automatically generated file, do not modify! -->

# Example Schema Reference

* Source file: [`examples/schema.json`](https://github.com/woozymasta/schemadoc/blob/HEAD/examples/schema.json)
* Source URL: [Raw schema URL](https://raw.githubusercontent.com/woozymasta/schemadoc/HEAD/examples/schema.json)
* Schema identifier: `https://github.com/woozymasta/schemadoc/schema-model`
* JSON Schema version: `https://json-schema.org/draft/2020-12/schema`
* Version support: `supported (2020-12)`
* Root reference: `#/$defs/SchemaModel`

## Contents

* [SchemaModel](#schemamodel)
  * [DraftInfo](#draftinfo)
  * [Options](#options)
* [Example yaml document](#example-yaml-document)

## SchemaModel

SchemaModel is the schema root for public package models.

| Attribute | Value |
| --- | --- |
| Type | `object` |
| Properties | 2 |
| Additional properties | boolean schema=false |

### SchemaModel.Options

Key: `options`

Options configures markdown generation.

| Attribute | Value |
| --- | --- |
| Required | yes |
| Reference | [`Options`](#options) (`#/$defs/Options`) |

### SchemaModel.DraftInfo

Key: `draft_info`

DraftInfo is the normalized output of draft detection.

| Attribute | Value |
| --- | --- |
| Required | yes |
| Reference | [`DraftInfo`](#draftinfo) (`#/$defs/DraftInfo`) |

## DraftInfo

DraftInfo describes detected JSON Schema draft support status.

| Attribute | Value |
| --- | --- |
| Type | `object` |
| Properties | 3 |
| Additional properties | boolean schema=false |

### DraftInfo.supported

Key: `supported`

Path: [`draft_info`](#schemamodeldraftinfo).`supported`

Supported reports whether draft is recognized by the renderer.

| Attribute | Value |
| --- | --- |
| Type | `boolean` |
| Required | yes |
| Default | `false` |

### DraftInfo.canonical

Key: `canonical`

Path: [`draft_info`](#schemamodeldraftinfo).`canonical`

Canonical is normalized draft alias (for example `2020-12`).

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `2020-12`, `draft-07` |

### DraftInfo.raw

Key: `raw`

Path: [`draft_info`](#schemamodeldraftinfo).`raw`

Raw is the original `$schema` value from input.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `https://json-schema.org/draft/2020-12/schema` |

## Options

Options configures markdown rendering behavior.

| Attribute | Value |
| --- | --- |
| Type | `object` |
| Properties | 13 |
| Additional properties | boolean schema=false |

### Options.description

Key: `description`

Path: [`options`](#schemamodeloptions).`description`

Description is optional top-level document description under title.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `Generated reference for runtime configuration schema.` |

### Options.example_format

Key: `example_format`

Path: [`options`](#schemamodeloptions).`example_format`

ExampleFormat enables optional embedded example payload in markdown templates
and selects encoding.

Supported values:

* `json`
* `yaml`

Empty value disables example embedding.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Enum | `json`, `yaml` |
| Examples | `json`, `yaml` |

### Options.example_mode

Key: `example_mode`

Path: [`options`](#schemamodeloptions).`example_mode`

ExampleMode controls property coverage for optional embedded example payload in
markdown templates.

Supported values:

* `all`
* `required`

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Enum | `all`, `required` |
| Examples | `all`, `required` |

### Options.footer_commit

Key: `footer_commit`

Path: [`options`](#schemamodeloptions).`footer_commit`

FooterCommit is optional footer build commit.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `abcdef1`, `unknown` |

### Options.footer_tool_name

Key: `footer_tool_name`

Path: [`options`](#schemamodeloptions).`footer_tool_name`

FooterToolName is optional footer tool label for rendered documents.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `schemadoc`, `lintkit` |

### Options.footer_tool_url

Key: `footer_tool_url`

Path: [`options`](#schemamodeloptions).`footer_tool_url`

FooterToolURL is optional footer tool project URL.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `https://github.com/woozymasta/schemadoc` |
| Format | `uri` |

### Options.footer_version

Key: `footer_version`

Path: [`options`](#schemamodeloptions).`footer_version`

FooterVersion is optional footer build version.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `v0.2.0`, `dev` |

### Options.list_marker

Key: `list_marker`

Path: [`options`](#schemamodeloptions).`list_marker`

ListMarker defines unordered markdown list marker used during description
normalization.

Supported values:

* `-`
* `*`

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Default | `*` |
| Enum | `-`, `*` |
| Examples | `*`, `-` |

### Options.source_path

Key: `source_path`

Path: [`options`](#schemamodeloptions).`source_path`

SourcePath is metadata shown in the document header.

It does not affect schema parsing, only rendered output.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `internal/config/schema.json`, `schemas/project.schema.json` |

### Options.template_name

Key: `template_name`

Path: [`options`](#schemamodeloptions).`template_name`

TemplateName selects one built-in template.

Supported values:

* `list`
* `table`
* `html`

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Default | `list` |
| Enum | `list`, `table`, `html` |
| Examples | `list`, `table`, `html` |

### Options.template_text

Key: `template_text`

Path: [`options`](#schemamodeloptions).`template_text`

TemplateText overrides built-in templates with custom template text.

Use this for project-specific markdown layouts.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Examples | `# {{ .Title }} Generated by custom template.` |

### Options.title

Key: `title`

Path: [`options`](#schemamodeloptions).`title`

Title is the top-level markdown heading.

This value is rendered as `# <title>`.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Default | `schema reference` |
| Examples | `schema reference`, `My Project Config Reference` |
| Constraints | minLength=1 |

### Options.wrap_width

Key: `wrap_width`

Path: [`options`](#schemamodeloptions).`wrap_width`

WrapWidth defines word-wrap width for plain description paragraphs.

Markdown structures such as lists, blockquotes, and fenced code blocks are
preserved.

| Attribute | Value |
| --- | --- |
| Type | `integer` |
| Required | no |
| Default | `80` |
| Examples | `80`, `100` |
| Constraints | minimum=1 |

## Example yaml document

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/woozymasta/schemadoc/HEAD/examples/schema.json

# DraftInfo is the normalized output of draft detection.
draft_info:
  # Canonical is normalized draft alias (for example `2020-12`).
  canonical: 2020-12
  # Raw is the original `$schema` value from input.
  raw: https://json-schema.org/draft/2020-12/schema
  # Supported reports whether draft is recognized by the renderer.
  supported: false
# Options configures markdown generation.
options:
  # Description is optional top-level document description under title.
  description: Generated reference for runtime configuration schema.
  # ExampleFormat enables optional embedded example payload in markdown templates and selects encoding.
  # Supported values:
  #  - `json`
  #  - `yaml`
  # Empty value disables example embedding.
  example_format: json
  # ExampleMode controls property coverage for optional embedded example payload in markdown templates.
  # Supported values:
  #  - `all`
  #  - `required`
  example_mode: all
  # FooterCommit is optional footer build commit.
  footer_commit: abcdef1
  # FooterToolName is optional footer tool label for rendered documents.
  footer_tool_name: schemadoc
  # FooterToolURL is optional footer tool project URL.
  footer_tool_url: https://github.com/woozymasta/schemadoc
  # FooterVersion is optional footer build version.
  footer_version: v0.2.0
  # ListMarker defines unordered markdown list marker used during description normalization.
  # Supported values:
  #  - `-`
  #  - `*`
  list_marker: '*'
  # SourcePath is metadata shown in the document header.
  # It does not affect schema parsing, only rendered output.
  source_path: internal/config/schema.json
  # TemplateName selects one built-in template.
  # Supported values:
  #  - `list`
  #  - `table`
  #  - `html`
  template_name: list
  # TemplateText overrides built-in templates with custom template text.
  # Use this for project-specific markdown layouts.
  template_text: |-
    # {{ .Title }}

    Generated by custom template.
  # Title is the top-level markdown heading.
  # This value is rendered as `# <title>`.
  title: schema reference
  # WrapWidth defines word-wrap width for plain description paragraphs.
  # Markdown structures such as lists, blockquotes, and fenced code blocks are preserved.
  wrap_width: 80
```

---

> Generated with
> [schemadoc](https://github.com/woozymasta/schemadoc)
> version `dev`
> commit `unknown`

<!-- Automatically generated file, do not modify! -->
