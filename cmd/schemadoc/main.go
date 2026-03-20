// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// schemadoc generates CommonMark docs from JSON Schema.
package main

import (
	_ "embed"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/woozymasta/schemadoc"
)

const (
	// helperModuleSuffix is appended to target module path for temporary helper module.
	helperModuleSuffix = "/schemadoc_mod2schema_helper"
	// jsonschemaDependency pins dependency used by temporary schema generator.
	jsonschemaDependency = "github.com/invopop/jsonschema@v0.13.0"
)

//go:embed templates/mod2schema_helper.go.tmpl
var mod2schemaHelperTemplate string

var schemaGeneratorProgramTemplate = template.Must(
	template.New("mod2schema_helper").Parse(mod2schemaHelperTemplate),
)

// cliOptions describes schemadoc CLI flags and subcommands.
type cliOptions struct {
	Version          versionCommand          `command:"version" description:"Print version information"`
	ModuleToSchema   moduleToSchemaCommand   `command:"mod2schema" description:"Generate JSON Schema from Go module type"`
	SchemaToJSON     schemaToJSONCommand     `command:"schema2json" description:"Generate example JSON payload from schema"`
	SchemaToYAML     schemaToYAMLCommand     `command:"schema2yaml" description:"Generate example YAML payload from schema"`
	Template         templateCommand         `command:"template" description:"Print built-in markdown template"`
	ModuleToMarkdown moduleToMarkdownCommand `command:"mod2md" description:"Generate markdown from Go module type"`
	SchemaToMarkdown schemaToMarkdownCommand `command:"schema2md" description:"Convert JSON Schema to markdown"`
}

// moduleReflectFlags groups common module reflection flags.
type moduleReflectFlags struct {
	ModuleRootPath string `short:"r" long:"module-root" description:"Filesystem path to module root (where go.mod is); used as working dir" default:"."`
	PackagePath    string `short:"p" long:"package" description:"Go package import path where the type is declared (optional; defaults to module argument)"`
	TypeName       string `short:"y" long:"type" description:"Go type name to reflect into schema (for example: Config)" required:"yes"`
	KeyNamer       string `long:"key-namer" description:"Optional reflected key naming strategy for fields without json tags" choice:"none" choice:"snake" choice:"kebab" choice:"lower" default:"none"`
}

// markdownRenderFlags groups markdown rendering flags.
type markdownRenderFlags struct {
	TemplatePath string `short:"f" long:"template-file" description:"Path to custom markdown template (.gotmpl)"`
	Title        string `short:"T" long:"title" description:"Markdown document title" default:"schema reference"`
	Description  string `short:"d" long:"description" description:"Optional top-level document description under title"`
	ListMarker   string `short:"l" long:"list-marker" description:"Unordered list marker for normalized descriptions" choice:"-" choice:"*" default:"*"`
	WrapWidth    int    `short:"w" long:"wrap" description:"Wrap width for plain text descriptions" default:"80"`
}

// templateSelectFlags groups built-in template selection flags.
type templateSelectFlags struct {
	TemplateName string `short:"t" long:"template" description:"Built-in template style" choice:"list" choice:"table" choice:"html" default:"list"`
}

// exampleModeFlags groups example mode flags.
type exampleModeFlags struct {
	Mode string `short:"m" long:"mode" description:"Example generation mode" choice:"all" choice:"required" default:"all"`
}

// markdownExampleFlags groups embedded example mode and format flags.
type markdownExampleFlags struct {
	Mode   string `short:"m" long:"mode" description:"Embedded example mode for markdown output" choice:"all" choice:"required" default:"all"`
	Format string `short:"F" long:"format" description:"Embedded example format for markdown output (empty disables embedding)" choice:"json" choice:"yaml"`
}

// moduleToMarkdownCommand wraps module-to-schema and schema-to-markdown flows.
type moduleToMarkdownCommand struct {
	runner *cliRunner

	ModuleFlags moduleReflectFlags `group:"Module Reflection"`
	Args        struct {
		Module string `positional-arg-name:"module" description:"Go module import path (for example: github.com/acme/project)" required:"yes"`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags  markdownExampleFlags `group:"Embedded Example"`
	TemplateFlags templateSelectFlags  `group:"Template Select"`
	RenderFlags   markdownRenderFlags  `group:"Markdown Render"`
}

// Execute runs mod2md subcommand.
func (command *moduleToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runModuleToMarkdown(
		moduleSchemaOptions{
			ModulePath:     command.Args.Module,
			TypeName:       command.ModuleFlags.TypeName,
			PackagePath:    command.ModuleFlags.PackagePath,
			ModuleRootPath: command.ModuleFlags.ModuleRootPath,
			KeyNamer:       command.ModuleFlags.KeyNamer,
		},
		markdownRenderRequest{
			TemplateName: command.TemplateFlags.TemplateName,
			Title:        command.RenderFlags.Title,
			Description:  command.RenderFlags.Description,
			TemplatePath: command.RenderFlags.TemplatePath,
			WrapWidth:    command.RenderFlags.WrapWidth,
			ListMarker:   command.RenderFlags.ListMarker,
			ExampleMode:  command.ExampleFlags.Mode,
			ExampleFmt:   command.ExampleFlags.Format,
			OutputPath:   command.Args.Output,
		},
	)
}

// moduleToSchemaCommand generates JSON Schema from a Go module model.
type moduleToSchemaCommand struct {
	runner *cliRunner
	Args   struct {
		Module string `positional-arg-name:"module" description:"Go module import path (for example: github.com/acme/project)" required:"yes"`
		Output string `positional-arg-name:"output" description:"Output schema file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ModuleFlags moduleReflectFlags `group:"Module Reflection"`
}

// Execute runs mod2schema subcommand.
func (command *moduleToSchemaCommand) Execute(_ []string) error {
	return command.runner.runModuleToSchema(moduleSchemaOptions{
		ModulePath:     command.Args.Module,
		TypeName:       command.ModuleFlags.TypeName,
		PackagePath:    command.ModuleFlags.PackagePath,
		ModuleRootPath: command.ModuleFlags.ModuleRootPath,
		KeyNamer:       command.ModuleFlags.KeyNamer,
	}, command.Args.Output)
}

// schemaToMarkdownCommand converts schema JSON to markdown.
type schemaToMarkdownCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input" description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags  markdownExampleFlags `group:"Embedded Example"`
	TemplateFlags templateSelectFlags  `group:"Template Select"`
	RenderFlags   markdownRenderFlags  `group:"Markdown Render"`
}

// Execute runs schemadoc subcommand.
func (command *schemaToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runSchemaToMarkdown(schemaMarkdownRequest{
		InputPath: command.Args.Input,
		Render: markdownRenderRequest{
			TemplateName: command.TemplateFlags.TemplateName,
			Title:        command.RenderFlags.Title,
			Description:  command.RenderFlags.Description,
			TemplatePath: command.RenderFlags.TemplatePath,
			WrapWidth:    command.RenderFlags.WrapWidth,
			ListMarker:   command.RenderFlags.ListMarker,
			ExampleMode:  command.ExampleFlags.Mode,
			ExampleFmt:   command.ExampleFlags.Format,
			OutputPath:   command.Args.Output,
		},
	})
}

// schemaToJSONCommand generates example JSON payload from schema.
type schemaToJSONCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input" description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output json file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags exampleModeFlags `group:"Example Generate"`
}

// Execute runs schema2json subcommand.
func (command *schemaToJSONCommand) Execute(_ []string) error {
	return command.runner.runSchemaToExample(schemaExampleRequest{
		Mode:       command.ExampleFlags.Mode,
		Format:     string(schemadoc.ExampleFormatJSON),
		InputPath:  command.Args.Input,
		OutputPath: command.Args.Output,
	})
}

// schemaToYAMLCommand generates example YAML payload from schema.
type schemaToYAMLCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input" description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output yaml file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	ExampleFlags exampleModeFlags `group:"Example Generate"`
}

// Execute runs schema2yaml subcommand.
func (command *schemaToYAMLCommand) Execute(_ []string) error {
	return command.runner.runSchemaToExample(schemaExampleRequest{
		Mode:       command.ExampleFlags.Mode,
		Format:     string(schemadoc.ExampleFormatYAML),
		InputPath:  command.Args.Input,
		OutputPath: command.Args.Output,
	})
}

// templateCommand exports built-in markdown template.
type templateCommand struct {
	runner *cliRunner
	Args   struct {
		Output string `positional-arg-name:"output" description:"Output template file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	TemplateFlags templateSelectFlags `group:"Template Select"`
}

// Execute runs template subcommand.
func (command *templateCommand) Execute(_ []string) error {
	return command.runner.runTemplate(command.TemplateFlags.TemplateName, command.Args.Output)
}

// cliRunner executes CLI operations with custom IO streams.
type cliRunner struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	programName string
}

// versionCommand prints version information.
type versionCommand struct {
}

// Execute runs version subcommand.
func (command *versionCommand) Execute(_ []string) error {
	printVersionInfo()
	return nil
}

// moduleSchemaOptions configures module-to-schema generation.
type moduleSchemaOptions struct {
	// ModulePath is the Go module path used by AddGoComments.
	ModulePath string
	// TypeName is the reflected root type name from target package.
	TypeName string
	// PackagePath is optional package import path and defaults to ModulePath.
	PackagePath string
	// ModuleRootPath is local working directory for go run and AddGoComments.
	ModuleRootPath string
	// KeyNamer is optional reflected key naming strategy.
	KeyNamer string
}

// schemaGeneratorTemplateData provides values for helper source template.
type schemaGeneratorTemplateData struct {
	// PackagePath is import path for reflected target package.
	PackagePath string
	// KeyNamer controls reflected key naming strategy.
	KeyNamer string
	// ModulePath is base module import path for AddGoComments.
	ModulePath string
	// ModuleRootPath is local module root path for comments normalization.
	ModuleRootPath string
	// TypeName is reflected root type.
	TypeName string
}

func init() {
	if _buildTime != "" {
		if t, err := time.Parse(time.RFC3339, _buildTime); err == nil {
			BuildTime = t.UTC()
		}
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes CLI logic and returns process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	return runWithIO(args, os.Stdin, stdout, stderr)
}

// runWithIO executes CLI logic with custom stdin, for tests.
func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	programName := strings.TrimSpace(os.Args[0])
	if programName == "" {
		programName = "schemadoc"
	}

	programName = filepath.Base(programName)
	runner := cliRunner{
		programName: programName,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
	}

	return runner.run(args)
}

// run parses CLI args and maps errors to process exit codes.
func (runner *cliRunner) run(args []string) int {
	err := parseCLIArgs(args, runner)
	if err == nil {
		return 0
	}

	var flagErr *flags.Error
	if errors.As(err, &flagErr) {
		if flagErr.Type == flags.ErrHelp {
			writeCLIError(runner.stdout, err)
			return 0
		}

		writeCLIError(runner.stderr, err)
		return 2
	}

	writeCLIError(runner.stderr, err)
	return 1
}

// parseCLIArgs parses CLI arguments and triggers selected subcommand execution.
func parseCLIArgs(args []string, runner *cliRunner) error {
	options := &cliOptions{}
	options.ModuleToMarkdown.runner = runner
	options.ModuleToSchema.runner = runner
	options.SchemaToMarkdown.runner = runner
	options.SchemaToJSON.runner = runner
	options.SchemaToYAML.runner = runner
	options.Template.runner = runner

	parser := flags.NewParser(options, flags.HelpFlag)
	parser.Name = runner.programName
	applyCommandLongDescriptions(parser, runner.programName)

	_, err := parser.ParseArgs(args)
	if err != nil {
		return err
	}

	return nil
}
