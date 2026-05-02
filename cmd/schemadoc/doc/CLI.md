<!-- markdownlint-disable MD013 MD024 MD036 -->
# schemadoc

## NAME

**schemadoc**

## SYNOPSIS

`schemadoc [OPTIONS]`

## Table of Contents

- [DESCRIPTION](#description)
- [OPTIONS](#options)
- [COMMANDS](#commands)
  - [help](#help)
  - [version](#version)
  - [build](#build)
  - [completion](#completion)
  - [config](#config)
  - [docs](#docs)
  - [docs html](#docs-html)
  - [docs man](#docs-man)
  - [docs md](#docs-md)
  - [merge](#merge)
  - [mod2doc](#mod2doc)
  - [mod2schema](#mod2schema)
  - [schema2doc](#schema2doc)
  - [schema2json](#schema2json)
  - [schema2yaml](#schema2yaml)
  - [template](#template)

## DESCRIPTION

schemadoc helps you build JSON Schema, docs, and example configs.You can generate docs from schema files, reflect Go types into schema,merge schema fragments, and run multi-step jobs from config.

## OPTIONS

### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

## COMMANDS

**Help Commands**

### help

Show help

**Usage:** `schemadoc [OPTIONS] help`

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

### version

Show version information

**Usage:** `schemadoc [OPTIONS] version`

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

### completion

Generate shell completion

**Usage:** `schemadoc [OPTIONS] completion [completion-OPTIONS]`

#### Generate shell completion

|Option|Description|Required|
|---|---|---|
|`--shell SHELL`|Shell completion format; choices: `bash, zsh, pwsh`|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output file path|no|

### docs

Generate documentation

**Usage:** `schemadoc [OPTIONS] docs`

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

### docs html

Generate HTML documentation

**Usage:** `schemadoc [OPTIONS] docs html [html-OPTIONS]`

#### Generate HTML documentation

|Option|Description|Default|Required|
|---|---|---|---|
|`--template TEMPLATE`|HTML documentation template; choices: `default, styled`|default|no|
|`--program-name NAME`|Override program name used in generated documentation templates||no|
|`--toc`|Include table of contents in output||no|
|`--trim-descriptions`|Trim description whitespace in generated output||no|
|`--include-hidden`|Include hidden options, groups and commands||no|
|`--mark-hidden`|Mark hidden entities in documentation output||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output file path|no|

### docs man

Generate man page documentation

**Usage:** `schemadoc [OPTIONS] docs man [man-OPTIONS]`

#### Generate man page documentation

|Option|Description|Required|
|---|---|---|
|`--program-name NAME`|Override program name used in generated documentation templates|no|
|`--trim-descriptions`|Trim description whitespace in generated output|no|
|`--include-hidden`|Include hidden options, groups and commands|no|
|`--mark-hidden`|Mark hidden entities in documentation output|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output file path|no|

### docs md

Generate Markdown documentation

**Usage:** `schemadoc [OPTIONS] docs md [md-OPTIONS]`

#### Generate Markdown documentation

|Option|Description|Default|Required|
|---|---|---|---|
|`--template TEMPLATE`|Markdown documentation template; choices: `list, table, code`|list|no|
|`--program-name NAME`|Override program name used in generated documentation templates||no|
|`--toc`|Include table of contents in output||no|
|`--trim-descriptions`|Trim description whitespace in generated output||no|
|`--include-hidden`|Include hidden options, groups and commands||no|
|`--mark-hidden`|Mark hidden entities in documentation output||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output file path|no|

### build

Run jobs from config file

Run jobs from YAML config.
When config path is omitted, ./schemadoc.build.yaml is used.
Index mode:

- --index 0 runs all documents in order.
- --index 1..N runs selected document.

Examples:

- `schemadoc build`
- `schemadoc build schemadoc.build.yaml --index 1`

**Usage:** `schemadoc [OPTIONS] build [build-OPTIONS]`

#### Run jobs from config file

|Option|Description|Default|Required|
|---|---|---|---|
|`-i`, `--index`|Config document index: 0 runs all documents, 1..N runs one document|0|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`config`|Config path (optional; default: ./schemadoc.build.yaml)|no|

### config

Generate config example

Generate config example. Useful for quick start and CI bootstrap.

Examples:

- `schemadoc config > schemadoc.build.yaml`
- `schemadoc config --mode required > schemadoc.build.min.yaml`

**Usage:** `schemadoc [OPTIONS] config [config-OPTIONS]`

#### Generate config example

|Option|Description|Default|Required|
|---|---|---|---|
|`-m`, `--mode`|Example generation mode; choices: `all, required`|all|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output file path (optional; stdout when omitted)|no|

### merge

Merge JSON Schema documents

Merge JSON Schema documents.
Supports direct map flags (--replace/--merge/--merge-defs) and config file (--config).

Examples:

- `schemadoc merge base.schema.json out.schema.json --merge-defs '/$defs=lint.schema.json#/$defs'`
- `schemadoc merge --config schema-merge.yaml --inplace`
- `schemadoc merge --config schema-merge.yaml --check`

**Usage:** `schemadoc [OPTIONS] merge [merge-OPTIONS]`

#### Schema Merge

|Option|Description|Required|
|---|---|---|
|`--replace <target=source>`|Replace target node: `<target-pointer>=<source-file[#/pointer]>`|no|
|`--merge <target=source>`|Deep-merge object node: `<target-pointer>=<source-file[#/pointer]>`|no|
|`--merge-defs <target=source>`|Merge source object fields into target object: `<target-pointer>=<source-file[#/pointer]>`|no|
|`--config`|Merge config file path (yaml/json)|no|
|`-f`, `--format`|Output format (inferred from output extension when omitted); choices: `json, yaml`|no|
|`--array-op`|Array mode for CLI map operations; choices: `replace, append, append-unique`|no|
|`--object-op`|Object mode for CLI map operations; choices: `merge, replace`|no|
|`-c`, `--check`|Check rendered output against output file and exit non-zero on diff|no|
|`--inplace`|Write result to source schema path when output path is not provided|no|
|`--prune-unreachable-defs`|Remove unreachable entries from $defs after merge|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`input`|Input schema path (optional; overrides config source)|no|
|`output`|Output schema path (optional; overrides config target)|no|

### mod2doc

Generate docs from Go type

Generate docs directly from Go type.
This is mod2schemaschema2doc in one command.
Use the same module/package/type selection rules as mod2schema.
Target package must be importable (package main is not supported).
Use --format json|yaml to append example payload code block at the end.

Examples:

- `schemadoc mod2doc --type Config . > model.md`
- `schemadoc mod2doc -t table --type Config github.com/acme/project@v1.2.3 docs/model.table.md`
- `schemadoc mod2doc -t html --description "Generated by CI." --type Config github.com/acme/project@v1.2.3 > model.html`
- `schemadoc mod2doc --mode required --format json --type Config github.com/acme/project@v1.2.3 > model.with-example.md`
- `schemadoc mod2doc --type Config --key-namer snake . > model.snake.md`

**Usage:** `schemadoc [OPTIONS] mod2doc [mod2doc-OPTIONS]`

#### Module Reflection

|Option|Description|Default|Required|
|---|---|---|---|
|`-p`, `--package`|Go package import path where type is declared (optional; default: module path)||no|
|`-y`, `--type`|Go type name (for example: Config)||yes|
|`--key-namer`|Field name style for fields without explicit json tags; choices: `none, snake, kebab, lower`|none|no|

#### Embedded Example

|Option|Description|Default|Required|
|---|---|---|---|
|`-m`, `--mode`|Embedded example mode for markdown output; choices: `all, required`|all|no|
|`-F`, `--format`|Embedded example format (omit to disable embedding); choices: `json, yaml`||no|

#### Template Select

|Option|Description|Default|Required|
|---|---|---|---|
|`-t`, `--template`|Built-in template style; choices: `list, table, html`|list|no|

#### Markdown Render

|Option|Description|Default|Required|
|---|---|---|---|
|`-f`, `--template-file`|Path to custom markdown template (.gotmpl)||no|
|`-T`, `--title`|Markdown document title|schema reference|no|
|`-d`, `--description`|Optional top-level document description under title||no|
|`-l`, `--list-marker`|List marker used in generated markdown lists; choices: `-, *`|*|no|
|`-w`, `--wrap`|Wrap width for plain text descriptions|80|no|
|`--hide-extra-keywords`|Hide non-standard schema keywords in Attributes||no|

#### JSON Output

|Option|Description|Default|Required|
|---|---|---|---|
|`--json-indent-type`|JSON indentation type; choices: `space, tab`|space|no|
|`--json-indent`|JSON indentation width|2|no|
|`--json-minify`|Write compact minified JSON output||no|

#### YAML Output

|Option|Description|Default|Required|
|---|---|---|---|
|`--yaml-indent`|YAML indentation width|2|no|
|`--disable-example-comments`|Disable YAML key comments from schema metadata||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`module`|Local module directory or remote module@version (optional; default: .)|no|
|`output`|Output markdown file path (optional; stdout when omitted)|no|

### mod2schema

Generate JSON Schema from Go type

Generate JSON Schema from Go type.
Pass local module dir or remote module@version as positional argument.
Use --package when type lives outside module root package.
Target package must be importable (package main is not supported).

Examples:

- `schemadoc mod2schema --type Config . > schema.json`
- `schemadoc mod2schema --package github.com/acme/project/internal/config --type Config github.com/acme/project@v1.2.3 schema.json`
- `schemadoc mod2schema --type Config --key-namer snake github.com/acme/project@latest > schema.snake.json`

**Usage:** `schemadoc [OPTIONS] mod2schema [mod2schema-OPTIONS]`

#### Module Reflection

|Option|Description|Default|Required|
|---|---|---|---|
|`-p`, `--package`|Go package import path where type is declared (optional; default: module path)||no|
|`-y`, `--type`|Go type name (for example: Config)||yes|
|`--key-namer`|Field name style for fields without explicit json tags; choices: `none, snake, kebab, lower`|none|no|

#### JSON Format

|Option|Description|Default|Required|
|---|---|---|---|
|`--json-indent-type`|JSON indentation type; choices: `space, tab`|space|no|
|`--json-indent`|JSON indentation width|2|no|
|`--json-minify`|Write compact minified JSON output||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`module`|Local module directory or remote module@version (optional; default: .)|no|
|`output`|Output schema file path (optional; stdout when omitted)|no|

### schema2doc

Generate docs from JSON Schema

Generate markdown or HTML docs from JSON Schema (--template).
Reads schema from file argument or stdin; writes output to file argument or stdout.
Use --format json|yaml to append example payload code block at the end.

Examples:

- `schemadoc schema2doc schema.json > schema.md`
- `cat schema.json | schemadoc schema2doc -t table > schema.table.md`
- `schemadoc schema2doc -t html --description "Generated by CI." schema.json > schema.html`
- `schemadoc schema2doc --mode required --format yaml schema.json > schema.with-example.md`

**Usage:** `schemadoc [OPTIONS] schema2doc [schema2doc-OPTIONS]`

#### Embedded Example

|Option|Description|Default|Required|
|---|---|---|---|
|`-m`, `--mode`|Embedded example mode for markdown output; choices: `all, required`|all|no|
|`-F`, `--format`|Embedded example format (omit to disable embedding); choices: `json, yaml`||no|

#### Template Select

|Option|Description|Default|Required|
|---|---|---|---|
|`-t`, `--template`|Built-in template style; choices: `list, table, html`|list|no|

#### Markdown Render

|Option|Description|Default|Required|
|---|---|---|---|
|`-f`, `--template-file`|Path to custom markdown template (.gotmpl)||no|
|`-T`, `--title`|Markdown document title|schema reference|no|
|`-d`, `--description`|Optional top-level document description under title||no|
|`-l`, `--list-marker`|List marker used in generated markdown lists; choices: `-, *`|*|no|
|`-w`, `--wrap`|Wrap width for plain text descriptions|80|no|
|`--hide-extra-keywords`|Hide non-standard schema keywords in Attributes||no|

#### JSON Output

|Option|Description|Default|Required|
|---|---|---|---|
|`--json-indent-type`|JSON indentation type; choices: `space, tab`|space|no|
|`--json-indent`|JSON indentation width|2|no|
|`--json-minify`|Write compact minified JSON output||no|

#### YAML Output

|Option|Description|Default|Required|
|---|---|---|---|
|`--yaml-indent`|YAML indentation width|2|no|
|`--disable-example-comments`|Disable YAML key comments from schema metadata||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`input`|Input schema file path (optional; stdin when omitted)|no|
|`output`|Output markdown file path (optional; stdout when omitted)|no|

### schema2json

Generate example JSON payload from schema

Generate example JSON payload from JSON Schema.
Reads schema from file argument or stdin; writes JSON to file argument or stdout.

Examples:

- `schemadoc schema2json schema.json > example.json`
- `schemadoc schema2json --mode required schema.json example.required.json`

**Usage:** `schemadoc [OPTIONS] schema2json [schema2json-OPTIONS]`

#### Example Generate

|Option|Description|Default|Required|
|---|---|---|---|
|`-m`, `--mode`|Example generation mode; choices: `all, required`|all|no|

#### JSON Format

|Option|Description|Default|Required|
|---|---|---|---|
|`--json-indent-type`|JSON indentation type; choices: `space, tab`|space|no|
|`--json-indent`|JSON indentation width|2|no|
|`--json-minify`|Write compact minified JSON output||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`input`|Input schema file path (optional; stdin when omitted)|no|
|`output`|Output json file path (optional; stdout when omitted)|no|

### schema2yaml

Generate example YAML payload from schema

Generate example YAML payload from JSON Schema.
Reads schema from file argument or stdin; writes YAML to file argument or stdout.

Examples:

- `schemadoc schema2yaml schema.json > example.yaml`
- `schemadoc schema2yaml --mode all schema.json example.all.yaml`

**Usage:** `schemadoc [OPTIONS] schema2yaml [schema2yaml-OPTIONS]`

#### Example Generate

|Option|Description|Default|Required|
|---|---|---|---|
|`-m`, `--mode`|Example generation mode; choices: `all, required`|all|no|

#### YAML Output

|Option|Description|Default|Required|
|---|---|---|---|
|`--yaml-indent`|YAML indentation width|2|no|
|`--disable-example-comments`|Disable YAML key comments from schema metadata||no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`input`|Input schema file path (optional; stdin when omitted)|no|
|`output`|Output yaml file path (optional; stdout when omitted)|no|

### template

Print built-in markdown template

Print built-in template text (list, table, or html).
Use it as a starting point for a custom template file.

Examples:

- `schemadoc template > list.gotmpl`
- `schemadoc template -t table templates/table.gotmpl`
- `schemadoc template -t html templates/reference.html.gotmpl`

**Usage:** `schemadoc [OPTIONS] template [template-OPTIONS]`

#### Template Select

|Option|Description|Default|Required|
|---|---|---|---|
|`-t`, `--template`|Built-in template style; choices: `list, table, html`|list|no|

#### Help Options

|Option|Description|Required|
|---|---|---|
|`-h`, `--help`|Show this help message|no|

#### Arguments

|Name|Description|Required|
|---|---|---|
|`output`|Output template file path (optional; stdout when omitted)|no|
