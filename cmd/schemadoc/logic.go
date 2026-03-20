// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

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

	"github.com/woozymasta/schemadoc"
)

// markdownRenderRequest stores markdown rendering request parameters.
type markdownRenderRequest struct {
	TemplateName string
	Title        string
	Description  string
	TemplatePath string
	ListMarker   string
	ExampleMode  string
	ExampleFmt   string
	OutputPath   string
	WrapWidth    int
}

// schemaMarkdownRequest stores schema-to-markdown request parameters.
type schemaMarkdownRequest struct {
	InputPath string
	Render    markdownRenderRequest
}

// schemaExampleRequest stores schema example generation request parameters.
type schemaExampleRequest struct {
	Mode       string
	Format     string
	InputPath  string
	OutputPath string
}

// schemaRenderPayload stores prepared schema bytes and source marker.
type schemaRenderPayload struct {
	SourcePath  string
	SchemaBytes []byte
}

// runModuleToMarkdown executes module-to-markdown flow without temporary schema files.
func (runner *cliRunner) runModuleToMarkdown(moduleOptions moduleSchemaOptions, render markdownRenderRequest) error {
	schemaBytes, sourcePath, err := generateModuleSchema(moduleOptions)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	return runner.runSchemaToMarkdownBytes(schemaRenderPayload{
		SchemaBytes: schemaBytes,
		SourcePath:  sourcePath,
	}, render)
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
func (runner *cliRunner) runSchemaToMarkdown(request schemaMarkdownRequest) error {
	schemaBytes, sourcePath, err := runner.readSchemaInput(request.InputPath)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	return runner.runSchemaToMarkdownBytes(schemaRenderPayload{
		SchemaBytes: schemaBytes,
		SourcePath:  sourcePath,
	}, request.Render)
}

// runSchemaToExample generates example payload for selected mode and format.
func (runner *cliRunner) runSchemaToExample(request schemaExampleRequest) error {
	schemaBytes, _, err := runner.readSchemaInput(request.InputPath)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	selectedMode, err := resolveExampleMode(request.Mode)
	if err != nil {
		return err
	}

	selectedFormat, err := resolveExampleFormat(request.Format)
	if err != nil {
		return err
	}

	content, err := schemadoc.GenerateExample(schemaBytes, selectedMode, selectedFormat)
	if err != nil {
		return fmt.Errorf("generate %s %s example: %w", selectedMode, selectedFormat, err)
	}

	outputPath := strings.TrimSpace(request.OutputPath)
	if outputPath == "" {
		if _, err := runner.stdout.Write(content); err != nil {
			return fmt.Errorf("write example to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		return fmt.Errorf("write example file %q: %w", outputPath, err)
	}

	return nil
}

// runSchemaToMarkdownBytes renders markdown from schema bytes and writes result to stdout or file.
func (runner *cliRunner) runSchemaToMarkdownBytes(payload schemaRenderPayload, render markdownRenderRequest) error {
	draftURI := extractSchemaDraftURI(payload.SchemaBytes)
	draft := schemadoc.DetectDraft(draftURI)
	if strings.TrimSpace(draftURI) == "" {
		_, _ = fmt.Fprintln(runner.stderr, "warning: schema has no $schema value; draft support is unknown")
	} else if !draft.Supported {
		_, _ = fmt.Fprintf(runner.stderr, "warning: unsupported $schema value %q\n", draftURI)
	}

	mode, format, err := resolveMarkdownExampleOptions(render.ExampleMode, render.ExampleFmt)
	if err != nil {
		return err
	}

	renderOptions := schemadoc.Options{
		Title:          render.Title,
		Description:    render.Description,
		SourcePath:     payload.SourcePath,
		TemplateName:   render.TemplateName,
		WrapWidth:      render.WrapWidth,
		ListMarker:     render.ListMarker,
		ExampleMode:    mode,
		ExampleFormat:  format,
		FooterToolName: "schemadoc",
		FooterToolURL:  URL,
		FooterVersion:  Version,
		FooterCommit:   Commit,
	}

	if render.TemplatePath != "" {
		customTemplate, err := os.ReadFile(render.TemplatePath)
		if err != nil {
			return fmt.Errorf("read template file %q: %w", render.TemplatePath, err)
		}

		renderOptions.TemplateText = string(customTemplate)
	}

	rendered, err := schemadoc.Render(payload.SchemaBytes, renderOptions)
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}

	if strings.TrimSpace(render.OutputPath) == "" {
		if _, err := io.WriteString(runner.stdout, rendered); err != nil {
			return fmt.Errorf("write markdown to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(render.OutputPath, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("write markdown file %q: %w", render.OutputPath, err)
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

// resolveExampleMode validates CLI mode flag value.
func resolveExampleMode(mode string) (schemadoc.ExampleMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all":
		return schemadoc.ExampleModeAll, nil
	case "required":
		return schemadoc.ExampleModeRequired, nil
	default:
		return "", fmt.Errorf("unsupported example mode %q", mode)
	}
}

// resolveExampleFormat validates CLI format flag value.
func resolveExampleFormat(format string) (schemadoc.ExampleFormat, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return schemadoc.ExampleFormatJSON, nil
	case "yaml":
		return schemadoc.ExampleFormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported example format %q", format)
	}
}

// resolveMarkdownExampleOptions parses optional markdown embedded example options.
func resolveMarkdownExampleOptions(modeRaw, formatRaw string) (schemadoc.ExampleMode, schemadoc.ExampleFormat, error) {
	formatRaw = strings.TrimSpace(formatRaw)
	if formatRaw == "" {
		return "", "", nil
	}

	mode, err := resolveExampleMode(modeRaw)
	if err != nil {
		return "", "", err
	}

	format, err := resolveExampleFormat(formatRaw)
	if err != nil {
		return "", "", err
	}

	return mode, format, nil
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

	options.KeyNamer = strings.ToLower(strings.TrimSpace(options.KeyNamer))
	if options.KeyNamer == "" {
		options.KeyNamer = "none"
	}

	return options
}

// buildSchemaGeneratorProgram renders temporary Go source used to reflect target module type.
func buildSchemaGeneratorProgram(options moduleSchemaOptions) string {
	var out bytes.Buffer
	data := schemaGeneratorTemplateData{
		PackagePath:    options.PackagePath,
		KeyNamer:       options.KeyNamer,
		ModulePath:     options.ModulePath,
		ModuleRootPath: options.ModuleRootPath,
		TypeName:       options.TypeName,
	}

	if err := schemaGeneratorProgramTemplate.Execute(&out, data); err != nil {
		panic(fmt.Sprintf("render mod2schema helper template: %v", err))
	}

	return out.String()
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

	if err := applySourceModuleReplaces(helperDir, options.ModuleRootPath, options.ModulePath); err != nil {
		return err
	}

	return nil
}

// applySourceModuleReplaces copies replace directives from source module go.mod.
func applySourceModuleReplaces(helperDir, moduleRootPath, targetModulePath string) error {
	replaces, err := listSourceModuleReplaces(moduleRootPath)
	if err != nil {
		return err
	}

	activeReplaces, err := listActiveModuleReplaces(moduleRootPath)
	if err != nil {
		return err
	}

	replaces = append(replaces, activeReplaces...)
	seen := make(map[string]struct{}, len(replaces))

	for index := range replaces {
		item := replaces[index]
		replaceArg, ok, buildErr := buildReplaceEditArg(item, moduleRootPath, targetModulePath)
		if buildErr != nil {
			return buildErr
		}

		if !ok {
			continue
		}

		if _, exists := seen[replaceArg]; exists {
			continue
		}

		seen[replaceArg] = struct{}{}
		if err := runGoCommand(helperDir, "mod", "edit", replaceArg); err != nil {
			return fmt.Errorf("apply source module replace %q: %w", replaceArg, err)
		}
	}

	return nil
}

// listSourceModuleReplaces reads replace directives from source module go.mod.
func listSourceModuleReplaces(moduleRootPath string) ([]goModEditReplace, error) {
	command := exec.Command("go", "mod", "edit", "-json")
	command.Dir = moduleRootPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return nil, fmt.Errorf("read source module go.mod replaces: %s", detail)
	}

	var payload goModEditJSON
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("decode source module go.mod replaces: %w", err)
	}

	return payload.Replace, nil
}

// listActiveModuleReplaces reads active replacements from module graph.
func listActiveModuleReplaces(moduleRootPath string) ([]goModEditReplace, error) {
	modCachePath, err := goEnvValue(moduleRootPath, "GOMODCACHE")
	if err != nil {
		return nil, err
	}

	moduleRootAbs, err := filepath.Abs(moduleRootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve module root %q: %w", moduleRootPath, err)
	}

	moduleRootAbs = filepath.Clean(moduleRootAbs)
	moduleParent := filepath.Dir(moduleRootAbs)
	modCachePath = filepath.Clean(strings.TrimSpace(modCachePath))

	command := exec.Command("go", "list", "-m", "-json", "all")
	command.Dir = moduleRootPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return nil, fmt.Errorf("read active module replaces: %s", detail)
	}

	decoder := json.NewDecoder(&stdout)
	replaces := make([]goModEditReplace, 0, 16)
	for {
		var item goListModule
		if err := decoder.Decode(&item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("decode active module replaces: %w", err)
		}

		modulePath := strings.TrimSpace(item.Path)
		if modulePath == "" {
			continue
		}

		newPath, newVersion, ok := resolveActiveReplaceTarget(
			item,
			modCachePath,
			moduleRootAbs,
			moduleParent,
		)
		if !ok {
			continue
		}

		replaces = append(replaces, goModEditReplace{
			Old: goModEditModule{
				Path:    modulePath,
				Version: strings.TrimSpace(item.Version),
			},
			New: goModEditModule{
				Path:    newPath,
				Version: newVersion,
			},
		})
	}

	return replaces, nil
}

// resolveActiveReplaceTarget resolves replacement target from go list module item.
func resolveActiveReplaceTarget(
	item goListModule,
	modCachePath string,
	moduleRootAbs string,
	moduleParent string,
) (string, string, bool) {
	if item.Replace != nil {
		newPath := strings.TrimSpace(item.Replace.Path)
		if newPath == "" {
			return "", "", false
		}

		newVersion := strings.TrimSpace(item.Replace.Version)
		// For local replace targets use absolute replacement dir from go list.
		if newVersion == "" {
			if dir := strings.TrimSpace(item.Replace.Dir); dir != "" {
				if absoluteDir, ok := toExistingAbsolutePath(dir); ok {
					newPath = filepath.ToSlash(absoluteDir)
				}
			}
		}

		return newPath, newVersion, true
	}

	dir := strings.TrimSpace(item.Dir)
	if dir == "" {
		return "", "", false
	}

	if !isLikelyLocalModuleDir(dir, modCachePath, moduleRootAbs, moduleParent) {
		return "", "", false
	}

	absoluteDir, ok := toExistingAbsolutePath(dir)
	if !ok {
		return "", "", false
	}

	return filepath.ToSlash(absoluteDir), "", true
}

// goEnvValue returns `go env <name>` output for selected module root.
func goEnvValue(moduleRootPath, name string) (string, error) {
	command := exec.Command("go", "env", name)
	command.Dir = moduleRootPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return "", fmt.Errorf("read go env %s: %s", name, detail)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// isLikelyLocalModuleDir reports whether dir should be copied as local replace.
func isLikelyLocalModuleDir(dir, modCachePath, moduleRootAbs, moduleParent string) bool {
	absoluteDir, ok := toExistingAbsolutePath(dir)
	if !ok {
		return false
	}

	if modCachePath != "" && strings.HasPrefix(strings.ToLower(absoluteDir), strings.ToLower(modCachePath)) {
		return false
	}

	if _, err := os.Stat(filepath.Join(absoluteDir, "go.mod")); err != nil {
		return false
	}

	if strings.EqualFold(absoluteDir, moduleRootAbs) {
		return true
	}

	return strings.HasPrefix(
		strings.ToLower(absoluteDir),
		strings.ToLower(moduleParent+string(os.PathSeparator)),
	)
}

// toExistingAbsolutePath normalizes path and verifies that it exists.
func toExistingAbsolutePath(value string) (string, bool) {
	absolutePath, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}

	absolutePath = filepath.Clean(absolutePath)
	if absolutePath == "" {
		return "", false
	}

	if _, err := os.Stat(absolutePath); err != nil {
		return "", false
	}

	return absolutePath, true
}

// buildReplaceEditArg converts one source replace into `go mod edit -replace=` argument.
func buildReplaceEditArg(
	item goModEditReplace,
	moduleRootPath string,
	targetModulePath string,
) (string, bool, error) {
	oldPath := strings.TrimSpace(item.Old.Path)
	if oldPath == "" {
		return "", false, nil
	}

	// Target module replace is already added explicitly.
	if oldPath == strings.TrimSpace(targetModulePath) {
		return "", false, nil
	}

	oldVersion := strings.TrimSpace(item.Old.Version)
	newPath := strings.TrimSpace(item.New.Path)
	if newPath == "" {
		return "", false, nil
	}

	newVersion := strings.TrimSpace(item.New.Version)
	if newVersion == "" && isRelativeOrAbsolutePath(newPath) {
		resolved := newPath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(moduleRootPath, resolved)
		}

		absolute, err := filepath.Abs(resolved)
		if err != nil {
			return "", false, fmt.Errorf("resolve replace path %q: %w", newPath, err)
		}

		newPath = filepath.ToSlash(absolute)
	}

	left := oldPath
	if oldVersion != "" {
		left += "@" + oldVersion
	}

	right := newPath
	if newVersion != "" {
		right += "@" + newVersion
	}

	return "-replace=" + left + "=" + right, true, nil
}

// isRelativeOrAbsolutePath reports whether replace target looks like filesystem path.
func isRelativeOrAbsolutePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	if filepath.IsAbs(trimmed) {
		return true
	}

	return strings.HasPrefix(trimmed, "./") ||
		strings.HasPrefix(trimmed, "../") ||
		strings.HasPrefix(trimmed, ".\\") ||
		strings.HasPrefix(trimmed, "..\\")
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
