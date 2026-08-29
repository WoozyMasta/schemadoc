// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// Package buildcfg defines the YAML build pipeline model for schemadoc CLI.
package buildcfg

/* ! NOTE FOR CONTRIBUTORS:
* Comments on exported types and fields in this file are end-user
* documentation. They are used to generate JSON Schema descriptions
* (IDE hints/autocomplete) and Markdown docs.
 */

// Config describes one build pipeline document.
//
// Each stage is optional. Build executes declared stages in fixed order:
// mod2schema -> merge -> schema2json -> schema2doc -> schema2yaml.
// Stage order does not depend on key order in YAML.
type Config struct {
	// Mod2Schema reflects one Go type into JSON Schema and writes result to
	// `schema`.
	//
	// When declared, this stage always runs first. Use it when schema must be
	// generated from source code before merge/export stages.
	Mod2Schema *Mod2SchemaStage `json:"mod2schema,omitempty" yaml:"mod2schema,omitempty" jsonschema_extras:"x-order=10"`

	// Merge applies import/patch actions over current `schema` file.
	//
	// When both mod2schema and merge are declared, merge always runs after
	// mod2schema. Use it to import shared definitions or patch fragments from
	// other schema files.
	Merge *MergeStage `json:"merge,omitempty" yaml:"merge,omitempty" jsonschema_extras:"x-order=20"`

	// Schema2JSON generates JSON example payload from current `schema`.
	//
	// This stage is useful for machine-readable fixtures and JSON examples in
	// repositories, tests, and CI artifacts.
	Schema2JSON *Schema2JSONStage `json:"schema2json,omitempty" yaml:"schema2json,omitempty" jsonschema_extras:"x-order=30"`

	// Schema2Doc renders documentation (list/table/html) from current `schema`.
	//
	// Use this stage for user-facing reference docs committed to repository.
	Schema2Doc *Schema2DocStage `json:"schema2doc,omitempty" yaml:"schema2doc,omitempty" jsonschema_extras:"x-order=40"`

	// Schema2YAML generates YAML example payload from current `schema`.
	//
	// This stage is useful for human-readable config examples and snippets.
	Schema2YAML *Schema2YAMLStage `json:"schema2yaml,omitempty" yaml:"schema2yaml,omitempty" jsonschema_extras:"x-order=50"`

	// Schema is working schema path for the whole document pipeline.
	//
	// `build` writes mod2schema output to this file, `merge` mutates this
	// file in place, and schema2* stages read from this file as input.
	// Use relative path for repository-local workflows, or absolute path
	// for CI/temp directories.
	Schema string `json:"schema" yaml:"schema" jsonschema:"required,minLength=1,example=schema.json,example=build/schema/config.schema.json" jsonschema_extras:"x-order=1"`

	// Check enables validation mode for all stages in the document.
	//
	// When true, build compares generated outputs with existing files and
	// fails on first mismatch instead of rewriting files. This is intended for
	// CI to ensure generated files are up to date.
	Check bool `json:"check,omitempty" yaml:"check,omitempty" jsonschema_extras:"x-order=2"`
}

// JSONOutputOptions configures JSON output formatting.
type JSONOutputOptions struct {
	// IndentType sets indentation character for pretty JSON.
	//
	// `space` uses spaces. `tab` uses tabs.
	IndentType string `json:"indent_type,omitempty" yaml:"indent_type,omitempty" jsonschema:"enum=space,enum=tab,default=space" jsonschema_extras:"x-order=2"`

	// Indent sets indentation width for one nesting level.
	//
	// For `space`, this is number of spaces. For `tab`, this is number of tabs.
	Indent int `json:"indent,omitempty" yaml:"indent,omitempty" jsonschema:"minimum=1,default=2" jsonschema_extras:"x-order=1"`

	// Minify enables compact one-line JSON output without indentation.
	Minify bool `json:"minify,omitempty" yaml:"minify,omitempty" jsonschema_extras:"x-order=3"`
}

// YAMLOutputOptions configures YAML output formatting and comments.
type YAMLOutputOptions struct {
	// Indent sets YAML indentation width for one nesting level.
	Indent int `json:"indent,omitempty" yaml:"indent,omitempty" jsonschema:"minimum=1,default=2" jsonschema_extras:"x-order=1"`

	// DisableExampleComments disables schema-driven comments above YAML keys.
	//
	// When false, generator writes comments from schema metadata (description,
	// default, example, enum).
	DisableExampleComments bool `json:"disable_example_comments,omitempty" yaml:"disable_example_comments,omitempty" jsonschema_extras:"x-order=2"`
}

// Mod2SchemaStage describes schema generation from one Go type.
type Mod2SchemaStage struct {
	// JSON configures formatting of generated schema file.
	JSON *JSONOutputOptions `json:"json,omitempty" yaml:"json,omitempty" jsonschema_extras:"x-order=5"`

	// Module selects reflection source.
	//
	// Local mode: existing directory on disk with go.mod.
	// Remote mode: module path with explicit version suffix
	// (`@vX.Y.Z` or `@latest`).
	Module string `json:"module,omitempty" yaml:"module,omitempty" jsonschema:"default=.,example=../project,example=github.com/acme/project@v1.2.3" jsonschema_extras:"x-order=1"`

	// Package is import path where target type is declared.
	//
	// Keep empty when type is declared in module root package.
	Package string `json:"package,omitempty" yaml:"package,omitempty" jsonschema:"example=github.com/acme/project/pkg/config" jsonschema_extras:"x-order=2"`

	// Type is root Go type name to reflect into schema.
	Type string `json:"type" yaml:"type" jsonschema:"required,minLength=1,example=Config,example=RuntimeOptions" jsonschema_extras:"x-order=3"`

	// KeyNamer controls fallback field-name strategy when struct field has no
	// explicit `json` tag.
	KeyNamer string `json:"key_namer,omitempty" yaml:"key_namer,omitempty" jsonschema:"enum=none,enum=snake,enum=kebab,enum=lower,default=none" jsonschema_extras:"x-order=4"`
}

// MergeStage describes merge execution over working schema.
type MergeStage struct {
	// Patches lists low-level merge actions applied in order.
	//
	// Each patch copies or merges one source node from another schema file
	// into current `schema`. Items are applied top-to-bottom; later patches
	// can overwrite results of earlier patches.
	Patches []MergePatch `json:"patches,omitempty" yaml:"patches,omitempty" jsonschema_extras:"x-order=1"`

	// Imports lists high-level definition imports applied in order.
	//
	// Each item imports entries from source defs object (for example `/$defs`)
	// into target defs object in current `schema` (also usually `/$defs`).
	// This is the convenient way to "pull shared type definitions" from other
	// schema files without writing many manual patches.
	Imports []MergeImport `json:"imports,omitempty" yaml:"imports,omitempty" jsonschema_extras:"x-order=2"`

	// PruneUnreachableDefs removes unreachable `$defs` entries after merge.
	PruneUnreachableDefs bool `json:"prune_unreachable_defs,omitempty" yaml:"prune_unreachable_defs,omitempty" jsonschema_extras:"x-order=3"`
}

// MergePatch describes one low-level merge action.
type MergePatch struct {
	// File is source schema path used by this action.
	File string `json:"file" yaml:"file" jsonschema:"required,minLength=1,example=schemas/common.schema.json" jsonschema_extras:"x-order=1"`

	// Source points to source node in source schema.
	//
	// Empty value means source schema root.
	Source string `json:"source,omitempty" yaml:"source,omitempty" jsonschema:"example=/$defs/CommonConfig" jsonschema_extras:"x-order=2"`

	// Target points to destination node in working schema.
	//
	// Empty value means working schema root.
	Target string `json:"target,omitempty" yaml:"target,omitempty" jsonschema:"example=/$defs/CommonConfig,example=/properties/spec" jsonschema_extras:"x-order=3"`

	// Op controls merge behavior for selected source and target nodes.
	Op MergeOp `json:"op" yaml:"op" jsonschema_extras:"x-order=4"`
}

// MergeImport describes one high-level definition import action.
type MergeImport struct {
	// File is source schema path from which definitions are imported.
	File string `json:"file" yaml:"file" jsonschema:"required,minLength=1,example=schemas/common.schema.json" jsonschema_extras:"x-order=1"`

	// SourceDefs selects source object used as definition map.
	//
	// Import reads all direct child keys from this pointer and treats each key
	// as one definition name to import.
	SourceDefs string `json:"source_defs,omitempty" yaml:"source_defs,omitempty" jsonschema:"default=/$defs,example=/components/schemas" jsonschema_extras:"x-order=2"`

	// TargetDefs selects destination object for imported definitions.
	//
	// Use this when project keeps reusable definitions outside default `/$defs`
	// or when imported definitions must be isolated under dedicated subtree.
	TargetDefs string `json:"target_defs,omitempty" yaml:"target_defs,omitempty" jsonschema:"default=/$defs,example=/components/schemas,example=/$defs/imported" jsonschema_extras:"x-order=3"`

	// Rename configures how imported definition names are transformed.
	//
	// Useful when source and target schemas use different naming conventions or
	// when name collisions must be avoided.
	Rename *MergeImportRename `json:"rename,omitempty" yaml:"rename,omitempty" jsonschema_extras:"x-order=4"`

	// Conflict sets behavior when target definition with same name already exists.
	Conflict string `json:"conflict,omitempty" yaml:"conflict,omitempty" jsonschema:"enum=replace,enum=merge,enum=keep,enum=error,default=error" jsonschema_extras:"x-order=5"`
}

// MergeImportRename configures imported definition name rewrite.
type MergeImportRename struct {
	// Mode selects rename strategy.
	//
	//   - `none` keeps original names.
	//   - `prefix` prepends `value`.
	//   - `suffix` appends `value`.
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=none,enum=prefix,enum=suffix,default=none" jsonschema_extras:"x-order=1"`

	// Value is prefix or suffix text used by selected rename mode.
	Value string `json:"value,omitempty" yaml:"value,omitempty" jsonschema:"example=model_,example=_shared" jsonschema_extras:"x-order=2"`
}

// MergeOp configures merge strategy for one patch action.
type MergeOp struct {
	// Node selects primary operation for target node.
	//
	//   - `replace` replaces target node with source node.
	//   - `merge` deep-merges source into target.
	//   - `merge-defs` merges fields of source object into target object.
	Node string `json:"node,omitempty" yaml:"node,omitempty" jsonschema:"enum=replace,enum=merge,enum=merge-defs,default=replace" jsonschema_extras:"x-order=1"`

	// Object selects object behavior for deep-merge operations.
	Object string `json:"object,omitempty" yaml:"object,omitempty" jsonschema:"enum=merge,enum=replace,default=merge" jsonschema_extras:"x-order=2"`

	// Array selects array behavior for deep-merge operations.
	Array string `json:"array,omitempty" yaml:"array,omitempty" jsonschema:"enum=replace,enum=append,enum=append-unique,default=replace" jsonschema_extras:"x-order=3"`
}

// Schema2DocStage describes documentation generation from working schema.
type Schema2DocStage struct {
	// JSON configures embedded JSON example formatting.
	//
	// Used when `format: json`.
	JSON *JSONOutputOptions `json:"json,omitempty" yaml:"json,omitempty" jsonschema_extras:"x-order=11"`

	// YAML configures embedded YAML example formatting.
	//
	// Used when `format: yaml`.
	YAML *YAMLOutputOptions `json:"yaml,omitempty" yaml:"yaml,omitempty" jsonschema_extras:"x-order=12"`

	// Output is markdown output path.
	//
	// When empty, build derives output from `schema` path:
	//   - template `list` -> `<schema-base>.list.md`
	//   - template `table` -> `<schema-base>.table.md`
	//   - template `html` -> `<schema-base>.html`
	Output string `json:"output,omitempty" yaml:"output,omitempty" jsonschema_extras:"x-order=1"`

	// Template selects built-in document template.
	Template string `json:"template,omitempty" yaml:"template,omitempty" jsonschema:"enum=list,enum=table,enum=html,default=list" jsonschema_extras:"x-order=2"`

	// TemplateFile points to custom template file and overrides `template`.
	//
	// Use this for project-specific layout and branding.
	TemplateFile string `json:"template_file,omitempty" yaml:"template_file,omitempty" jsonschema:"example=templates/custom.md.gotmpl" jsonschema_extras:"x-order=3"`

	// Title overrides top-level document heading.
	Title string `json:"title,omitempty" yaml:"title,omitempty" jsonschema:"example=schema reference,example=Project Config Reference" jsonschema_extras:"x-order=4"`

	// Description overrides top-level document description.
	Description string `json:"description,omitempty" yaml:"description,omitempty" jsonschema:"example=Generated by CI pipeline." jsonschema_extras:"x-order=5"`

	// ListMarker controls unordered list marker normalization in rendered markdown.
	ListMarker string `json:"list_marker,omitempty" yaml:"list_marker,omitempty" jsonschema:"enum=-,enum=*,default=*" jsonschema_extras:"x-order=6"`

	// Mode selects embedded example generation mode for doc template.
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=all,enum=required,default=all" jsonschema_extras:"x-order=7"`

	// Format selects embedded example format for doc template.
	//
	// Empty value disables embedded example block.
	Format string `json:"format,omitempty" yaml:"format,omitempty" jsonschema:"enum=json,enum=yaml,default=json" jsonschema_extras:"x-order=8"`

	// Wrap sets wrap width for plain-text description blocks.
	Wrap int `json:"wrap,omitempty" yaml:"wrap,omitempty" jsonschema:"minimum=1,default=80" jsonschema_extras:"x-order=9"`

	// HideExtraKeywords disables "Other keywords" attribute row in rendered docs.
	//
	// Internal keywords such as `x-order` are hidden regardless of this option.
	HideExtraKeywords bool `json:"hide_extra_keywords,omitempty" yaml:"hide_extra_keywords,omitempty" jsonschema_extras:"x-order=10"`

	// ShowInternalKeywords includes renderer-specific schema keywords in rendered docs.
	ShowInternalKeywords bool `json:"show_internal_keywords,omitempty" yaml:"show_internal_keywords,omitempty" jsonschema_extras:"x-order=11"`

	// Footer includes schemadoc version metadata in rendered documentation.
	Footer bool `json:"footer,omitempty" yaml:"footer,omitempty" jsonschema:"default=false" jsonschema_extras:"x-order=12"`
}

// Schema2JSONStage describes schema2json stage.
type Schema2JSONStage struct {
	// JSON configures output formatting for schema2json.
	JSON *JSONOutputOptions `json:"json,omitempty" yaml:"json,omitempty" jsonschema_extras:"x-order=3"`

	// Output is output file path.
	//
	// When empty, build derives output from `schema` path:
	//   - schema2json -> `<schema-base>.json`
	Output string `json:"output,omitempty" yaml:"output,omitempty" jsonschema_extras:"x-order=1"`

	// Mode sets example generation mode (`all` or `required`).
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=all,enum=required,default=all" jsonschema_extras:"x-order=2"`
}

// Schema2YAMLStage describes schema2yaml stage.
type Schema2YAMLStage struct {
	// YAML configures output formatting/comments for schema2yaml.
	YAML *YAMLOutputOptions `json:"yaml,omitempty" yaml:"yaml,omitempty" jsonschema_extras:"x-order=3"`

	// Output is output file path.
	//
	// When empty, build derives output from `schema` path:
	//   - schema2yaml -> `<schema-base>.yaml`
	Output string `json:"output,omitempty" yaml:"output,omitempty" jsonschema_extras:"x-order=1"`

	// Mode sets example generation mode (`all` or `required`).
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"enum=all,enum=required,default=all" jsonschema_extras:"x-order=2"`
}
