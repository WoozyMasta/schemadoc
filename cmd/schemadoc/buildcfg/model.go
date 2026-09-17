// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// Package buildcfg defines the YAML build pipeline model for schemadoc CLI.
package buildcfg

/* ! NOTE FOR CONTRIBUTORS:
 * Comments on exported types and fields in this file are end-user documentation.
 * They are used to generate JSON Schema descriptions (IDE hints/autocomplete) and Markdown docs.
 */

// Config describes one build pipeline document.
//
// Each stage is optional. Build executes declared stages in fixed order:
// mod2schema -> merge -> schema2json -> schema2doc -> schema2yaml.
// Stage order does not depend on key order in YAML.
type Config struct {
	// Mod2Schema reflects one Go type into JSON Schema and writes result to `schema`.
	//
	// When declared, this stage always runs first.
	// Use it when schema must be generated from source code before merge/export stages.
	Mod2Schema *Mod2SchemaStage `json:"mod2schema,omitempty" yaml:"mod2schema,omitempty" jsonschema_extras:"x-order=10"`

	// Merge applies import/patch actions over current `schema` file.
	//
	// When both mod2schema and merge are declared, merge always runs after mod2schema.
	// Use it to import shared definitions or patch fragments from other schema files.
	Merge *MergeStage `json:"merge,omitempty" yaml:"merge,omitempty" jsonschema_extras:"x-order=20"`

	// Schema2JSON generates JSON example payload from current `schema`.
	//
	// This stage is useful for machine-readable fixtures and JSON examples in repositories, tests, and CI artifacts.
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
	// `build` writes mod2schema output to this file, `merge` mutates this file in place,
	// and schema2* stages read from this file as input.
	// Use relative path for repository-local workflows, or absolute path for CI/temp directories.
	Schema string `json:"schema" yaml:"schema" jsonschema_extras:"x-order=1" jsonschema:"required,minLength=1,example=schema.json,example=build/schema/config.schema.json"`

	// Check enables validation mode for all stages in the document.
	//
	// When true, build compares generated outputs with existing files
	// and fails on first mismatch instead of rewriting files.
	// This is intended for CI to ensure generated files are up to date.
	Check bool `json:"check,omitempty" yaml:"check,omitempty" jsonschema_extras:"x-order=2"`
}

// JSONOutputOptions configures JSON output formatting.
type JSONOutputOptions struct {
	// IndentType sets indentation character for pretty JSON.
	//
	// `space` uses spaces. `tab` uses tabs.
	IndentType string `json:"indent_type,omitempty" yaml:"indent_type,omitempty" jsonschema_extras:"x-order=2" jsonschema:"enum=space,enum=tab,default=space"`

	// Indent sets indentation width for one nesting level.
	//
	// For `space`, this is number of spaces. For `tab`, this is number of tabs.
	Indent int `json:"indent,omitempty" yaml:"indent,omitempty" jsonschema_extras:"x-order=1" jsonschema:"minimum=1,default=2"`

	// Minify enables compact one-line JSON output without indentation.
	Minify bool `json:"minify,omitempty" yaml:"minify,omitempty" jsonschema_extras:"x-order=3"`
}

// YAMLCommentOptions configures independent schema annotation comments.
type YAMLCommentOptions struct {
	// Titles enables comments sourced from schema titles.
	Titles *bool `json:"titles,omitempty" yaml:"titles,omitempty" jsonschema_extras:"x-order=1" jsonschema:"default=true"`

	// Descriptions enables comments sourced from schema descriptions.
	Descriptions *bool `json:"descriptions,omitempty" yaml:"descriptions,omitempty" jsonschema_extras:"x-order=2" jsonschema:"default=true"`

	// Defaults enables comments sourced from schema defaults.
	Defaults *bool `json:"defaults,omitempty" yaml:"defaults,omitempty" jsonschema_extras:"x-order=3" jsonschema:"default=true"`

	// Enums enables comments listing schema enum values.
	Enums *bool `json:"enums,omitempty" yaml:"enums,omitempty" jsonschema_extras:"x-order=4" jsonschema:"default=true"`

	// Examples selects scalar, all, or no schema examples.
	Examples string `json:"examples,omitempty" yaml:"examples,omitempty" jsonschema_extras:"x-order=5" jsonschema:"enum=none,enum=scalar,enum=all,default=scalar"`

	// ExampleFormat selects inline or block annotation value formatting.
	ExampleFormat string `json:"example_format,omitempty" yaml:"example_format,omitempty" jsonschema_extras:"x-order=6" jsonschema:"enum=inline,enum=block,default=block"`

	// Spacing selects compact or paragraph-preserving annotation comments.
	Spacing string `json:"spacing,omitempty" yaml:"spacing,omitempty" jsonschema_extras:"x-order=7" jsonschema:"enum=compact,enum=full,default=compact"`
}

// YAMLOutputOptions configures YAML output formatting and comments.
type YAMLOutputOptions struct {
	// Comments configures schema-driven comments above YAML keys.
	Comments *YAMLCommentOptions `json:"comments,omitempty" yaml:"comments,omitempty" jsonschema_extras:"x-order=2"`

	// Indent sets YAML indentation width for one nesting level.
	Indent int `json:"indent,omitempty" yaml:"indent,omitempty" jsonschema_extras:"x-order=1" jsonschema:"minimum=1,default=2"`
}

// Mod2SchemaStage describes schema generation from one Go type.
type Mod2SchemaStage struct {
	// JSON configures formatting of generated schema file.
	JSON *JSONOutputOptions `json:"json,omitempty" yaml:"json,omitempty" jsonschema_extras:"x-order=6"`

	// Module selects reflection source.
	//
	// Local mode: existing directory on disk with go.mod.
	// Remote mode: module path with explicit version suffix (`@vX.Y.Z` or `@latest`).
	Module string `json:"module,omitempty" yaml:"module,omitempty" jsonschema_extras:"x-order=1" jsonschema:"default=.,example=../project,example=github.com/acme/project@v1.2.3"`

	// Package is import path where target type is declared.
	//
	// Keep empty when type is declared in module root package.
	Package string `json:"package,omitempty" yaml:"package,omitempty" jsonschema_extras:"x-order=2" jsonschema:"example=github.com/acme/project/pkg/config"`

	// Type is root Go type name to reflect into schema.
	Type string `json:"type" yaml:"type" jsonschema_extras:"x-order=3" jsonschema:"required,minLength=1,example=Config,example=RuntimeOptions"`

	// KeyNamer controls fallback field-name strategy when struct field has no explicit `json` tag.
	KeyNamer string `json:"key_namer,omitempty" yaml:"key_namer,omitempty" jsonschema_extras:"x-order=4" jsonschema:"enum=none,enum=snake,enum=kebab,enum=lower,default=none"`

	// RootID overrides the automatically generated root `$id` in the output JSON Schema.
	// It must be an absolute URI without a fragment. Leave it empty to use the generator's default.
	RootID string `json:"root_id,omitempty" yaml:"root_id,omitempty" jsonschema_extras:"x-order=5" jsonschema:"format=uri,example=https://example.com/schema.json"`
}

// MergeStage describes merge execution over working schema.
type MergeStage struct {
	// Patches lists low-level merge actions applied in order.
	//
	// Each patch copies or merges one source node from another schema file into current `schema`.
	// Items are applied top-to-bottom; later patches can overwrite results of earlier patches.
	Patches []MergePatch `json:"patches,omitempty" yaml:"patches,omitempty" jsonschema_extras:"x-order=1"`

	// Imports lists high-level definition imports applied in order.
	//
	// Each item imports entries from source defs object (for example `/$defs`)
	// into target defs object in current `schema` (also usually `/$defs`).
	// This is the convenient way to "pull shared type definitions" from other schema files without writing many manual patches.
	Imports []MergeImport `json:"imports,omitempty" yaml:"imports,omitempty" jsonschema_extras:"x-order=2"`

	// PruneUnreachableDefs removes unreachable `$defs` entries after merge.
	PruneUnreachableDefs bool `json:"prune_unreachable_defs,omitempty" yaml:"prune_unreachable_defs,omitempty" jsonschema_extras:"x-order=3"`
}

// MergePatch describes one low-level merge action.
type MergePatch struct {
	// File is source schema path used by this action.
	File string `json:"file" yaml:"file" jsonschema_extras:"x-order=1" jsonschema:"required,minLength=1,example=schemas/common.schema.json"`

	// Source points to source node in source schema.
	//
	// Empty value means source schema root.
	Source string `json:"source,omitempty" yaml:"source,omitempty" jsonschema_extras:"x-order=2" jsonschema:"example=/$defs/CommonConfig"`

	// Target points to destination node in working schema.
	//
	// Empty value means working schema root.
	Target string `json:"target,omitempty" yaml:"target,omitempty" jsonschema_extras:"x-order=3" jsonschema:"example=/$defs/CommonConfig,example=/properties/spec"`

	// Op controls merge behavior for selected source and target nodes.
	Op MergeOp `json:"op" yaml:"op" jsonschema_extras:"x-order=4"`
}

// MergeImport describes one high-level definition import action.
type MergeImport struct {
	// File is source schema path from which definitions are imported.
	File string `json:"file" yaml:"file" jsonschema_extras:"x-order=1" jsonschema:"required,minLength=1,example=schemas/common.schema.json"`

	// SourceDefs selects source object used as definition map.
	//
	// Import reads all direct child keys from this pointer and treats each key as one definition name to import.
	SourceDefs string `json:"source_defs,omitempty" yaml:"source_defs,omitempty" jsonschema_extras:"x-order=2" jsonschema:"default=/$defs,example=/components/schemas"`

	// TargetDefs selects destination object for imported definitions.
	//
	// Use this when project keeps reusable definitions outside default `/$defs`
	// or when imported definitions must be isolated under dedicated subtree.
	TargetDefs string `json:"target_defs,omitempty" yaml:"target_defs,omitempty" jsonschema_extras:"x-order=3" jsonschema:"default=/$defs,example=/components/schemas,example=/$defs/imported"`

	// Rename configures how imported definition names are transformed.
	//
	// Useful when source and target schemas use different naming conventions or when name collisions must be avoided.
	Rename *MergeImportRename `json:"rename,omitempty" yaml:"rename,omitempty" jsonschema_extras:"x-order=4"`

	// Conflict sets behavior when target definition with same name already exists.
	Conflict string `json:"conflict,omitempty" yaml:"conflict,omitempty" jsonschema_extras:"x-order=5" jsonschema:"enum=replace,enum=merge,enum=keep,enum=error,default=error"`
}

// MergeImportRename configures imported definition name rewrite.
type MergeImportRename struct {
	// Mode selects rename strategy.
	//
	//   - `none` keeps original names.
	//   - `prefix` prepends `value`.
	//   - `suffix` appends `value`.
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema_extras:"x-order=1" jsonschema:"enum=none,enum=prefix,enum=suffix,default=none"`

	// Value is prefix or suffix text used by selected rename mode.
	Value string `json:"value,omitempty" yaml:"value,omitempty" jsonschema_extras:"x-order=2" jsonschema:"example=model_,example=_shared"`
}

// MergeOp configures merge strategy for one patch action.
type MergeOp struct {
	// Node selects primary operation for target node.
	//
	//   - `replace` replaces target node with source node.
	//   - `merge` deep-merges source into target.
	//   - `merge-defs` merges fields of source object into target object.
	Node string `json:"node,omitempty" yaml:"node,omitempty" jsonschema_extras:"x-order=1" jsonschema:"enum=replace,enum=merge,enum=merge-defs,default=replace"`

	// Object selects object behavior for deep-merge operations.
	// It controls whether source object members are merged or replace the target.
	Object string `json:"object,omitempty" yaml:"object,omitempty" jsonschema_extras:"x-order=2" jsonschema:"enum=merge,enum=replace,default=merge"`

	// Array selects array behavior for deep-merge operations.
	// It controls whether arrays are replaced, appended, or appended uniquely.
	Array string `json:"array,omitempty" yaml:"array,omitempty" jsonschema_extras:"x-order=3" jsonschema:"enum=replace,enum=append,enum=append-unique,default=replace"`
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

	// Output is document output path.
	//
	// When empty, build derives output from `schema` path and template metadata.
	// Builtin list and table templates keep `.list` and `.table` suffixes;
	// custom templates use their declared `output.extension` directly.
	Output string `json:"output,omitempty" yaml:"output,omitempty" jsonschema_extras:"x-order=1"`

	// Template selects built-in document template.
	Template string `json:"template,omitempty" yaml:"template,omitempty" jsonschema_extras:"x-order=2" jsonschema:"enum=list,enum=table,enum=html,default=list"`

	// TemplateFile points to custom template file and overrides `template`.
	//
	// Use this for project-specific layout and branding.
	TemplateFile string `json:"template_file,omitempty" yaml:"template_file,omitempty" jsonschema_extras:"x-order=3" jsonschema:"example=templates/custom.md.gotmpl"`

	// Title overrides top-level document heading.
	Title string `json:"title,omitempty" yaml:"title,omitempty" jsonschema_extras:"x-order=4" jsonschema:"example=schema reference,example=Project Config Reference"`

	// Description overrides top-level document description.
	Description string `json:"description,omitempty" yaml:"description,omitempty" jsonschema_extras:"x-order=5" jsonschema:"example=Generated by CI pipeline."`

	// ListMarker controls unordered list marker normalization in rendered documents.
	ListMarker string `json:"list_marker,omitempty" yaml:"list_marker,omitempty" jsonschema_extras:"x-order=6" jsonschema:"enum=-,enum=*,default=*"`

	// Mode selects embedded example generation mode for doc template.
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema_extras:"x-order=7" jsonschema:"enum=all,enum=required,default=all"`

	// Format selects embedded example format for doc template.
	//
	// Empty value disables embedded example block.
	Format string `json:"format,omitempty" yaml:"format,omitempty" jsonschema_extras:"x-order=8" jsonschema:"enum=json,enum=yaml"`

	// Wrap sets wrap width for plain-text description blocks.
	Wrap int `json:"wrap,omitempty" yaml:"wrap,omitempty" jsonschema_extras:"x-order=9" jsonschema:"minimum=1,default=80"`

	// HideExtraKeywords disables "Other keywords" attribute row in rendered docs.
	//
	// Internal keywords such as `x-order` are hidden regardless of this option.
	HideExtraKeywords bool `json:"hide_extra_keywords,omitempty" yaml:"hide_extra_keywords,omitempty" jsonschema_extras:"x-order=10"`

	// ShowInternalKeywords includes renderer-specific schema keywords in rendered docs.
	ShowInternalKeywords bool `json:"show_internal_keywords,omitempty" yaml:"show_internal_keywords,omitempty" jsonschema_extras:"x-order=11"`

	// Footer includes schemadoc version metadata in rendered documentation.
	Footer bool `json:"footer,omitempty" yaml:"footer,omitempty" jsonschema_extras:"x-order=12" jsonschema:"default=false"`
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
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema_extras:"x-order=2" jsonschema:"enum=all,enum=required,default=all"`
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
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema_extras:"x-order=2" jsonschema:"enum=all,enum=required,default=all"`
}
