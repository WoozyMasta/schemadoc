// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// schemadoc generates CommonMark docs from JSON Schema.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

var (
	Version    = "dev"
	Commit     = "unknown"
	BuildTime  = time.Unix(0, 0)
	URL        = "https://github.com/woozymasta/schemadoc"
	_buildTime string
)

// cliOptions describes schemadoc CLI flags and subcommands.
type cliOptions struct {
	Version          versionCommand          `command:"version" description:"Print version information"`
	ModuleToSchema   moduleToSchemaCommand   `command:"mod2schema" description:"Generate JSON Schema from Go module type"`
	Template         templateCommand         `command:"template" description:"Print built-in markdown template"`
	ModuleToMarkdown moduleToMarkdownCommand `command:"mod2md" description:"Generate markdown from Go module type"`
	SchemaToMarkdown schemaToMarkdownCommand `command:"schema2md" description:"Convert JSON Schema to markdown"`
}

// moduleReflectFlags groups common module reflection flags.
type moduleReflectFlags struct {
	ModuleRootPath string `short:"r" long:"module-root" description:"Filesystem path to module root (where go.mod is); used as working dir" default:"."`
	PackagePath    string `short:"p" long:"package" description:"Go package import path where the type is declared (optional; defaults to module argument)"`
	TypeName       string `short:"y" long:"type" description:"Go type name to reflect into schema (for example: Config)" required:"yes"`
}

// markdownRenderFlags groups markdown rendering flags.
type markdownRenderFlags struct {
	TemplatePath string `short:"f" long:"template-file" description:"Path to custom markdown template (.gotmpl)"`
	Title        string `short:"T" long:"title" description:"Markdown document title" default:"schema reference"`
	ListMarker   string `short:"l" long:"list-marker" description:"Unordered list marker for normalized descriptions" choice:"-" choice:"*" default:"*"`
	WrapWidth    int    `short:"w" long:"wrap" description:"Wrap width for plain text descriptions" default:"80"`
}

// templateSelectFlags groups built-in template selection flags.
type templateSelectFlags struct {
	TemplateName string `short:"t" long:"template" description:"Built-in template style" choice:"list" choice:"table" default:"list"`
}

// moduleToMarkdownCommand wraps module-to-schema and schema-to-markdown flows.
type moduleToMarkdownCommand struct {
	runner *cliRunner

	ModuleFlags moduleReflectFlags `group:"Module Reflection"`
	Args        struct {
		Module string `positional-arg-name:"module" description:"Go module import path (for example: github.com/acme/project)" required:"yes"`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	TemplateFlags templateSelectFlags `group:"Template Select"`
	RenderFlags   markdownRenderFlags `group:"Markdown Render"`
}

// Execute runs mod2md subcommand.
func (command *moduleToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runModuleToMarkdown(
		moduleSchemaOptions{
			ModulePath:     command.Args.Module,
			TypeName:       command.ModuleFlags.TypeName,
			PackagePath:    command.ModuleFlags.PackagePath,
			ModuleRootPath: command.ModuleFlags.ModuleRootPath,
		},
		command.TemplateFlags.TemplateName,
		command.RenderFlags.Title,
		command.RenderFlags.TemplatePath,
		command.RenderFlags.WrapWidth,
		command.RenderFlags.ListMarker,
		command.Args.Output,
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
	}, command.Args.Output)
}

// schemaToMarkdownCommand converts schema JSON to markdown.
type schemaToMarkdownCommand struct {
	runner *cliRunner
	Args   struct {
		Input  string `positional-arg-name:"input" description:"Input schema file path (optional; stdin when omitted)"`
		Output string `positional-arg-name:"output" description:"Output markdown file path (optional; stdout when omitted)"`
	} `positional-args:"yes"`

	TemplateFlags templateSelectFlags `group:"Template Select"`

	RenderFlags markdownRenderFlags `group:"Markdown Render"`
}

// Execute runs schemadoc subcommand.
func (command *schemaToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runSchemaToMarkdown(
		command.TemplateFlags.TemplateName,
		command.RenderFlags.Title,
		command.RenderFlags.TemplatePath,
		command.RenderFlags.WrapWidth,
		command.RenderFlags.ListMarker,
		command.Args.Input,
		command.Args.Output,
	)
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

// runModuleToMarkdown executes module-to-markdown flow without temporary schema files.
func (runner *cliRunner) runModuleToMarkdown(moduleOptions moduleSchemaOptions, templateName, title, templatePath string, wrapWidth int, listMarker, outputPath string) error {
	schemaBytes, sourcePath, err := generateModuleSchema(moduleOptions)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	return runner.runSchemaToMarkdownBytes(templateName, title, templatePath, wrapWidth, listMarker, schemaBytes, sourcePath, outputPath)
}

// runModuleToSchema executes module-to-schema flow and writes result to stdout or file.
func (runner *cliRunner) runModuleToSchema(moduleOptions moduleSchemaOptions, outputPath string) error {
	schemaBytes, _, err := generateModuleSchema(moduleOptions)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	if strings.TrimSpace(outputPath) == "" {
		if _, err := runner.stdout.Write(schemaBytes); err != nil {
			return fmt.Errorf("write schema to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(outputPath, schemaBytes, 0o600); err != nil {
		return fmt.Errorf("write schema file %q: %w", outputPath, err)
	}

	return nil
}

// runSchemaToMarkdown executes schema-to-markdown flow and writes result to stdout or file.
func (runner *cliRunner) runSchemaToMarkdown(templateName, title, templatePath string, wrapWidth int, listMarker, inputPath, outputPath string) error {
	schemaBytes, sourcePath, err := runner.readSchemaInput(inputPath)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	return runner.runSchemaToMarkdownBytes(templateName, title, templatePath, wrapWidth, listMarker, schemaBytes, sourcePath, outputPath)
}

// runSchemaToMarkdownBytes renders markdown from schema bytes and writes result to stdout or file.
func (runner *cliRunner) runSchemaToMarkdownBytes(templateName, title, templatePath string, wrapWidth int, listMarker string, schemaBytes []byte, sourcePath, outputPath string) error {
	draftURI := extractSchemaDraftURI(schemaBytes)
	draft := schemadoc.DetectDraft(draftURI)
	if strings.TrimSpace(draftURI) == "" {
		_, _ = fmt.Fprintln(runner.stderr, "warning: schema has no $schema value; draft support is unknown")
	} else if !draft.Supported {
		_, _ = fmt.Fprintf(runner.stderr, "warning: unsupported $schema value %q\n", draftURI)
	}

	renderOptions := schemadoc.Options{
		Title:        title,
		SourcePath:   sourcePath,
		TemplateName: templateName,
		WrapWidth:    wrapWidth,
		ListMarker:   listMarker,
	}

	if templatePath != "" {
		customTemplate, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("read template file %q: %w", templatePath, err)
		}

		renderOptions.TemplateText = string(customTemplate)
	}

	rendered, err := schemadoc.Render(schemaBytes, renderOptions)
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}

	if strings.TrimSpace(outputPath) == "" {
		if _, err := io.WriteString(runner.stdout, rendered); err != nil {
			return fmt.Errorf("write markdown to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(outputPath, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("write markdown file %q: %w", outputPath, err)
	}

	return nil
}

// runTemplate writes selected built-in template to stdout or file.
func (runner *cliRunner) runTemplate(templateName, outputPath string) error {
	tpl, err := schemadoc.BuiltinTemplate(templateName)
	if err != nil {
		return fmt.Errorf("load built-in template %q: %w", templateName, err)
	}

	if strings.TrimSpace(outputPath) == "" {
		if _, err := io.WriteString(runner.stdout, tpl); err != nil {
			return fmt.Errorf("write template to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(outputPath, []byte(tpl), 0o600); err != nil {
		return fmt.Errorf("write template file %q: %w", outputPath, err)
	}

	return nil
}

// readSchemaInput reads schema from file path or stdin and returns source marker.
func (runner *cliRunner) readSchemaInput(path string) ([]byte, string, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read schema file %q: %w", path, err)
		}

		return data, path, nil
	}

	data, err := io.ReadAll(runner.stdin)
	if err != nil {
		return nil, "", fmt.Errorf("read schema from stdin: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, "", errors.New("read schema from stdin: empty input")
	}

	return data, "(stdin)", nil
}

// writeCLIError writes a plain-text CLI error line to the selected stream.
func writeCLIError(output io.Writer, err error) {
	if err == nil {
		return
	}

	//nolint:gosec // CLI writes plain-text diagnostics to terminal streams, not HTTP responses.
	_, _ = fmt.Fprintln(output, err.Error())
}

// parseCLIArgs parses CLI arguments and triggers selected subcommand execution.
func parseCLIArgs(args []string, runner *cliRunner) error {
	options := &cliOptions{}
	options.ModuleToMarkdown.runner = runner
	options.ModuleToSchema.runner = runner
	options.SchemaToMarkdown.runner = runner
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

// applyCommandLongDescriptions configures detailed command help text with examples.
func applyCommandLongDescriptions(parser *flags.Parser, programName string) {
	descriptions := map[string]string{
		"template": strings.TrimSpace(fmt.Sprintf(`
Print built-in markdown template text (`+"`list` or `table`"+`).
Use it as a starting point for a custom template file.

Examples:
> $ %s template > list.gotmpl
> $ %s template -t table templates/table.gotmpl
`, programName, programName)),
		"schemadoc": strings.TrimSpace(fmt.Sprintf(`
Convert JSON Schema to markdown.
Reads schema from file argument or stdin; writes markdown to file argument or stdout.

Examples:
> $ %s schemadoc schema.json > schema.md
> $ cat schema.json | %s schemadoc -t table > schema.table.md
`, programName, programName)),
		"mod2schema": strings.TrimSpace(fmt.Sprintf(`
Reflect Go type into JSON Schema.
Use module import path as positional argument.
Use --module-root for local module directory and --package when type is not in module root package.

Examples:
> $ %s mod2schema --module-root . --type Config github.com/acme/project > schema.json
> $ %s mod2schema --module-root . --package github.com/acme/project/internal/config --type Config github.com/acme/project schema.json
`, programName, programName)),
		"mod2md": strings.TrimSpace(fmt.Sprintf(`
Generate markdown directly from Go type.
This is `+"`mod2schema` + `schema2md`"+` in one command.
Use the same module/package/type selection rules as `+"`mod2schema`"+`.

Examples:
> $ %s mod2md --module-root . --type Config github.com/acme/project > model.md
> $ %s mod2md -t table --module-root . --type Config github.com/acme/project docs/model.table.md
`, programName, programName)),
	}

	for commandName, description := range descriptions {
		command := parser.Find(commandName)
		if command == nil {
			continue
		}

		command.LongDescription = description
	}
}

// extractSchemaDraftURI returns raw $schema value from schema document.
func extractSchemaDraftURI(schemaBytes []byte) string {
	var root map[string]any
	if err := json.Unmarshal(schemaBytes, &root); err != nil {
		return ""
	}

	value, ok := root["$schema"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(value)
}

// generateModuleSchema reflects JSON Schema for the selected module/package/type triple.
func generateModuleSchema(options moduleSchemaOptions) ([]byte, string, error) {
	normalizedOptions := normalizeModuleSchemaOptions(options)
	moduleRootPath, err := filepath.Abs(normalizedOptions.ModuleRootPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve module root path %q: %w", normalizedOptions.ModuleRootPath, err)
	}

	normalizedOptions.ModuleRootPath = filepath.ToSlash(moduleRootPath)

	if err := ensureGoToolchain(); err != nil {
		return nil, "", err
	}

	helperSource := buildSchemaGeneratorProgram(normalizedOptions)
	helperDir, err := writeSchemaGeneratorProgram(helperSource)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = os.RemoveAll(helperDir)
	}()

	if err := initSchemaGeneratorWorkspace(helperDir, normalizedOptions); err != nil {
		return nil, "", err
	}

	if err := installSchemaGeneratorDependencies(helperDir); err != nil {
		return nil, "", err
	}

	schemaBytes, err := runSchemaGeneratorProgram(helperDir)
	if err != nil {
		return nil, "", err
	}

	sourcePath := fmt.Sprintf("module:%s.%s", normalizedOptions.PackagePath, normalizedOptions.TypeName)
	return schemaBytes, sourcePath, nil
}

// normalizeModuleSchemaOptions normalizes module reflection options.
func normalizeModuleSchemaOptions(options moduleSchemaOptions) moduleSchemaOptions {
	options.ModulePath = strings.TrimSpace(options.ModulePath)
	options.TypeName = strings.TrimSpace(options.TypeName)
	options.PackagePath = strings.TrimSpace(options.PackagePath)
	if options.PackagePath == "" {
		options.PackagePath = options.ModulePath
	}

	options.ModuleRootPath = strings.TrimSpace(options.ModuleRootPath)
	if options.ModuleRootPath == "" {
		options.ModuleRootPath = "."
	}

	return options
}

// buildSchemaGeneratorProgram renders temporary Go source used to reflect target module type.
func buildSchemaGeneratorProgram(options moduleSchemaOptions) string {
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
	target %q
)

func normalizeCommentKeys(r *jsonschema.Reflector, base, root string) {
	if r == nil || r.CommentMap == nil {
		return
	}

	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	root = strings.ReplaceAll(strings.TrimSpace(root), "\\", "/")
	root = strings.TrimSuffix(root, "/")
	if base == "" || root == "" {
		return
	}

	prefix := base + "/" + strings.TrimPrefix(root, "/")
	normalized := make(map[string]string, len(r.CommentMap))
	for key, value := range r.CommentMap {
		normalizedKey := strings.ReplaceAll(key, "\\", "/")
		if strings.HasPrefix(normalizedKey, prefix) {
			normalizedKey = base + strings.TrimPrefix(normalizedKey, prefix)
		}

		normalized[normalizedKey] = value
	}

	r.CommentMap = normalized
}

func main() {
	reflector := new(jsonschema.Reflector)

	if err := reflector.AddGoComments(%q, %q); err != nil {
		fmt.Fprintf(os.Stderr, "add go comments: %%v\\n", err)
		os.Exit(1)
	}
	normalizeCommentKeys(reflector, %q, %q)

	schema := reflector.Reflect(&target.%s{})
	if schema == nil {
		fmt.Fprintln(os.Stderr, "reflect schema: empty result")
		os.Exit(1)
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal schema: %%v\\n", err)
		os.Exit(1)
	}

	data = append(data, '\n')
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "write schema to stdout: %%v\\n", err)
		os.Exit(1)
	}
}
`, options.PackagePath, options.ModulePath, options.ModuleRootPath, options.ModulePath, options.ModuleRootPath, options.TypeName)
}

// writeSchemaGeneratorProgram stores temporary source code in system temp directory.
func writeSchemaGeneratorProgram(source string) (string, error) {
	helperDir, err := os.MkdirTemp("", "schemadoc-mod2schema-")
	if err != nil {
		return "", fmt.Errorf("create temporary schema generator dir: %w", err)
	}

	helperPath := filepath.Join(helperDir, "main.go")
	if err := os.WriteFile(helperPath, []byte(source), 0o600); err != nil {
		return "", fmt.Errorf("write temporary schema generator: %w", err)
	}

	return helperDir, nil
}

// initSchemaGeneratorWorkspace initializes temporary go module for schema generation.
func initSchemaGeneratorWorkspace(helperDir string, options moduleSchemaOptions) error {
	helperModulePath := buildSchemaGeneratorModulePath(options.ModulePath)
	if err := runGoCommand(helperDir, "mod", "init", helperModulePath); err != nil {
		return fmt.Errorf("init temporary module: %w", err)
	}

	requireArg := "-require=" + options.ModulePath + "@v0.0.0"
	if err := runGoCommand(helperDir, "mod", "edit", requireArg); err != nil {
		return fmt.Errorf("require target module %q: %w", options.ModulePath, err)
	}

	replaceArg := "-replace=" + options.ModulePath + "=" + options.ModuleRootPath
	if err := runGoCommand(helperDir, "mod", "edit", replaceArg); err != nil {
		return fmt.Errorf("replace target module %q: %w", options.ModulePath, err)
	}

	return nil
}

// installSchemaGeneratorDependencies installs required helper module dependencies.
func installSchemaGeneratorDependencies(helperDir string) error {
	if err := runGoCommand(helperDir, "get", jsonschemaDependency); err != nil {
		return fmt.Errorf("install helper dependency %q: %w", jsonschemaDependency, err)
	}

	if err := runGoCommand(helperDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy helper module: %w", err)
	}

	return nil
}

// runSchemaGeneratorProgram executes temporary schema generator and returns reflected schema bytes.
func runSchemaGeneratorProgram(helperDir string) ([]byte, error) {
	command := exec.Command("go", "run", ".")
	command.Dir = helperDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return nil, fmt.Errorf("run module schema generator: %s", detail)
	}

	return stdout.Bytes(), nil
}

// buildSchemaGeneratorModulePath returns temporary helper module path for target module imports.
func buildSchemaGeneratorModulePath(modulePath string) string {
	return strings.TrimSuffix(strings.TrimSpace(modulePath), "/") + helperModuleSuffix
}

// ensureGoToolchain validates Go availability for mod2schema/mod2md flows.
func ensureGoToolchain() error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go toolchain not found in PATH; mod2schema and mod2md require installed Go")
	}

	return nil
}

// runGoCommand executes one Go command in selected directory and returns detailed error.
func runGoCommand(dir string, args ...string) error {
	command := exec.Command("go", args...)
	command.Dir = dir

	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(output.String())
		if detail == "" {
			detail = err.Error()
		}

		return fmt.Errorf("go %s: %s", strings.Join(args, " "), detail)
	}

	return nil
}

func printVersionInfo() {
	fmt.Printf(`url:      %s
file:     %s
version:  %s
commit:   %s
built:    %s
`, URL, os.Args[0], Version, Commit, BuildTime)
}
