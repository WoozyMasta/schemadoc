// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

// cliOptions describes schemadoc CLI flags and subcommands.
type cliOptions struct {
	Config           configCommand           `command:"config"      description:"Generate config example"`
	Template         templateCommand         `command:"template"    description:"Print built-in markdown template"`
	SchemaMerge      schemaMergeCommand      `command:"merge"       description:"Merge JSON Schema documents"`
	Build            buildCommand            `command:"build"       description:"Run jobs from config file"`
	ModuleToSchema   moduleToSchemaCommand   `command:"mod2schema"  description:"Generate JSON Schema from Go type"`
	SchemaToJSON     schemaToJSONCommand     `command:"schema2json" description:"Generate example JSON payload from schema"`
	SchemaToYAML     schemaToYAMLCommand     `command:"schema2yaml" description:"Generate example YAML payload from schema"`
	ModuleToMarkdown moduleToMarkdownCommand `command:"mod2doc"     description:"Generate docs from Go type"`
	SchemaToDoc      schemaToDocCommand      `command:"schema2doc"  description:"Generate docs from JSON Schema"`
}

// moduleReflectFlags groups common module reflection flags.
type moduleReflectFlags struct {
	PackagePath       string `long:"package"            description:"Go package import path where type is declared (optional; default: module path)" short:"p"`
	TypeName          string `long:"type"               description:"Go type name (for example: Config)" short:"y" required:"yes"`
	KeyNamer          string `long:"key-namer"          description:"Field name style for fields without explicit json tags" default:"none" choices:"none;snake;kebab;lower"`
	JSONSchemaVersion string `long:"jsonschema-version" description:"Override github.com/invopop/jsonschema version for helper module (for example: v0.14.0)"`
}

// markdownRenderFlags groups markdown rendering flags.
type markdownRenderFlags struct {
	TemplatePath         string `long:"template-file"          description:"Path to custom markdown template (.gotmpl)"           short:"f"`
	Title                string `long:"title"                  description:"Markdown document title"                              short:"T" default:"schema reference"`
	Description          string `long:"description"            description:"Optional top-level document description under title"  short:"d"`
	ListMarker           string `long:"list-marker"            description:"List marker used in generated markdown lists"         short:"l" default:"*" choices:"-;*"`
	WrapWidth            int    `long:"wrap"                   description:"Wrap width for plain text descriptions"               short:"w" default:"80" validate-min:"1"`
	HideExtraKeywords    bool   `long:"hide-extra-keywords"    description:"Hide non-standard schema keywords in Attributes"`
	ShowInternalKeywords bool   `long:"show-internal-keywords" description:"Show renderer-specific schema keywords in Attributes"`
	Footer               bool   `long:"footer"                 description:"Include schemadoc version footer in generated output"`
}

// templateSelectFlags groups built-in template selection flags.
type templateSelectFlags struct {
	TemplateName string `short:"t" default:"list" long:"template" choices:"list;table;html" description:"Built-in template style"`
}

// exampleModeFlags groups example mode flags.
type exampleModeFlags struct {
	Mode string `short:"m" long:"mode" default:"all" choices:"all;required" description:"Example generation mode"`
}

// jsonFormatFlags groups JSON output formatting flags.
type jsonFormatFlags struct {
	IndentType string `long:"json-indent-type" description:"JSON indentation type"              default:"space" choices:"space;tab"`
	Indent     int    `long:"json-indent"      description:"JSON indentation width"             default:"2" validate-min:"1"`
	Minify     bool   `long:"json-minify"      description:"Write compact minified JSON output"`
}

// yamlExampleFlags groups YAML example output flags.
type yamlExampleFlags struct {
	Indent                 int  `long:"yaml-indent"              description:"YAML indentation width" default:"2" validate-min:"1"`
	DisableExampleComments bool `long:"disable-example-comments" description:"Disable YAML key comments from schema metadata"`
}

// markdownExampleFlags groups embedded example mode and format flags.
type markdownExampleFlags struct {
	Mode   string `short:"m" long:"mode"   choices:"all;required" description:"Embedded example mode for markdown output" default:"all"`
	Format string `short:"F" long:"format" choices:"json;yaml"    description:"Embedded example format (omit to disable embedding)"`
}

// moduleToMarkdownCommand wraps module-to-schema and schema-to-markdown flows.
type moduleToMarkdownCommand struct {
	runner *cliRunner

	ModuleFlags moduleReflectFlags `group:"Module Reflection"`
	Args        struct {
		Module string `positional-arg-name:"module" description:"Local module directory or remote module@version (optional; default: .)" default:"."`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags  markdownExampleFlags `group:"Embedded Example"`
	TemplateFlags templateSelectFlags  `group:"Template Select"`
	RenderFlags   markdownRenderFlags  `group:"Markdown Render"`
	JSONFlags     jsonFormatFlags      `group:"JSON Output"`
	YAMLFlags     yamlExampleFlags     `group:"YAML Output"`
}

// moduleToSchemaCommand generates JSON Schema from a Go module model.
type moduleToSchemaCommand struct {
	runner *cliRunner
	Args   struct {
		Module string `positional-arg-name:"module" description:"Local module directory or remote module@version (optional; default: .)" default:"."`
		Output string `positional-arg-name:"output" description:"Output schema file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ModuleFlags moduleReflectFlags `group:"Module Reflection"`
	JSONFlags   jsonFormatFlags    `group:"JSON Format"`
}

// schemaToDocCommand converts schema JSON to docs.
type schemaToDocCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input"  description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags  markdownExampleFlags `group:"Embedded Example"`
	TemplateFlags templateSelectFlags  `group:"Template Select"`
	RenderFlags   markdownRenderFlags  `group:"Markdown Render"`
	JSONFlags     jsonFormatFlags      `group:"JSON Output"`
	YAMLFlags     yamlExampleFlags     `group:"YAML Output"`
}

// schemaToJSONCommand generates example JSON payload from schema.
type schemaToJSONCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input"  description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output json file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags exampleModeFlags `group:"Example Generate"`
	JSONFlags    jsonFormatFlags  `group:"JSON Format"`
}

// schemaToYAMLCommand generates example YAML payload from schema.
type schemaToYAMLCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input"  description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output yaml file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags exampleModeFlags `group:"Example Generate"`
	YAMLFlags    yamlExampleFlags `group:"YAML Output"`
}

// configCommand generates config example.
type configCommand struct {
	runner *cliRunner
	Args   struct {
		Output string `description:"Output file path (optional; stdout when omitted)" positional-arg-name:"output"`
	} `positional-args:"yes"`

	Mode string `description:"Example generation mode" short:"m" long:"mode" default:"all" choices:"all;required"`
}

// templateCommand exports built-in markdown template.
type templateCommand struct {
	runner *cliRunner
	Args   struct {
		Output string `positional-arg-name:"output" description:"Output template file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	TemplateFlags templateSelectFlags `group:"Template Select"`
}

// schemaMergeCommand merges schemas using replace/merge actions.
type schemaMergeCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input"  description:"Input schema path (optional; overrides config source)"`
		Output string `positional-arg-name:"output" description:"Output schema path (optional; overrides config target)"`
	} `positional-args:"yes"`

	MergeFlags schemaMergeFlags `group:"Schema Merge"`
}

// schemaMergeMap parses repeated key=value mappings for merge flags.
type schemaMergeMap map[string]string

// schemaMergeFlags groups merge CLI flags.
type schemaMergeFlags struct {
	Replace              schemaMergeMap `long:"replace"                description:"Replace target node: <target-pointer>=<source-file[#/pointer]>" value-name:"<target=source>"`
	Merge                schemaMergeMap `long:"merge"                  description:"Deep-merge object node: <target-pointer>=<source-file[#/pointer]>" value-name:"<target=source>"`
	MergeDefs            schemaMergeMap `long:"merge-defs"             description:"Merge source object fields into target object: <target-pointer>=<source-file[#/pointer]>" value-name:"<target=source>"`
	ConfigPath           string         `long:"config"                 description:"Merge config file path (yaml/json)"`
	Format               string         `long:"format"                 description:"Output format (inferred from output extension when omitted)" choices:"json;yaml" short:"f"`
	ArrayOp              string         `long:"array-op"               description:"Array mode for CLI map operations" choices:"replace;append;append-unique"`
	ObjectOp             string         `long:"object-op"              description:"Object mode for CLI map operations" choices:"merge;replace"`
	Check                bool           `long:"check"                  description:"Check rendered output against output file and exit non-zero on diff" short:"c"`
	InPlace              bool           `long:"inplace"                description:"Write result to source schema path when output path is not provided"`
	PruneUnreachableDefs bool           `long:"prune-unreachable-defs" description:"Remove unreachable entries from $defs after merge"`
}

// buildCommand executes schemadoc jobs from config document.
type buildCommand struct {
	runner *cliRunner
	Args   struct {
		ConfigPath string `description:"Config path (optional; default: ./schemadoc.build.yaml)" positional-arg-name:"config"`
	} `positional-args:"yes"`

	ConfigIndex int `description:"Config document index: 0 runs all documents, 1..N runs one document" short:"i" default:"0" long:"index" validate-min:"0"`
}
