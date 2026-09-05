// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	moduleSourceLocal  = "local"
	moduleSourceRemote = "remote"

	// schemaGeneratorJSONSchemaModule is helper dependency module path.
	schemaGeneratorJSONSchemaModule = "github.com/invopop/jsonschema"
	// schemaGeneratorJSONSchemaVersion is default pinned helper dependency.
	schemaGeneratorJSONSchemaVersion = "v0.14.0"
	// schemaGeneratorMinGoVersion is minimum Go version for pinned dependency.
	schemaGeneratorMinGoVersion = "1.24"
)

// Generate reflects JSON Schema for selected module/package/type.
func Generate(options Options) ([]byte, string, error) {
	normalizedOptions := NormalizeOptions(options)
	jsonSchemaVersion, hasCustomJSONSchemaVersion := resolveJSONSchemaVersion(
		normalizedOptions.JSONSchemaVersion,
	)

	resolvedTarget, err := resolveTarget(normalizedOptions)
	if err != nil {
		return nil, "", err
	}

	if err := ensureGoToolchain(); err != nil {
		return nil, "", err
	}

	if !hasCustomJSONSchemaVersion {
		if err := ensureGoVersionAtLeast(schemaGeneratorMinGoVersion); err != nil {
			return nil, "", err
		}
	}

	helperDir, err := createSchemaGeneratorDir()
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = os.RemoveAll(helperDir)
	}()

	if err := initSchemaGeneratorWorkspace(
		helperDir,
		resolvedTarget,
		jsonSchemaVersion,
	); err != nil {
		return nil, "", err
	}

	if err := ensureImportableTargetPackage(helperDir, resolvedTarget); err != nil {
		return nil, "", err
	}

	if resolvedTarget.Source == moduleSourceRemote {
		if err := preloadRemoteModule(helperDir, resolvedTarget); err != nil {
			return nil, "", err
		}

		resolvedTarget.ModuleDir, err = moduleDirFromHelperWorkspace(
			helperDir,
			resolvedTarget.ModulePath,
		)
		if err != nil {
			return nil, "", err
		}
	}

	helperSource, err := renderProgramSource(templateData{
		PackagePath: resolvedTarget.PackagePath,
		TypeName:    normalizedOptions.Type,
		ModulePath:  resolvedTarget.ModulePath,
		ModuleDir:   resolvedTarget.ModuleDir,
		KeyNamer:    normalizedOptions.KeyNamer,
	})
	if err != nil {
		return nil, "", err
	}

	if err := writeSchemaGeneratorProgram(helperDir, helperSource); err != nil {
		return nil, "", err
	}

	if err := prepareSchemaGeneratorDependencies(helperDir); err != nil {
		return nil, "", err
	}

	schemaBytes, err := runSchemaGeneratorProgram(helperDir)
	if err != nil {
		return nil, "", err
	}

	sourcePath := fmt.Sprintf(
		"module:%s.%s",
		resolvedTarget.PackagePath,
		normalizedOptions.Type,
	)
	return schemaBytes, sourcePath, nil
}

// ensureImportableTargetPackage validates package importability before helper run.
func ensureImportableTargetPackage(
	helperDir string,
	target resolvedTarget,
) error {
	packageName, err := packageNameFromHelperWorkspace(
		helperDir,
		target.PackagePath,
	)
	if err != nil {
		return err
	}

	if packageName == "main" {
		return fmt.Errorf(
			"%w: package %q resolves to package main; move the type to an importable package (for example: internal/config) and pass --package to that path",
			ErrMainPackageUnsupported,
			target.PackagePath,
		)
	}

	return nil
}

// preloadRemoteModule downloads target module to resolve local source dir.
func preloadRemoteModule(helperDir string, target resolvedTarget) error {
	if target.Source != moduleSourceRemote {
		return nil
	}

	if strings.TrimSpace(target.ModuleVer) == "" {
		return errors.New("remote module version is required")
	}

	targetRef := target.ModulePath + "@" + target.ModuleVer
	if err := runGoCommand(helperDir, "mod", "download", targetRef); err != nil {
		return fmt.Errorf("download remote module %q: %w", targetRef, err)
	}

	return nil
}

// NormalizeOptions normalizes module reflection options.
func NormalizeOptions(options Options) Options {
	options.Module = strings.TrimSpace(options.Module)
	if options.Module == "" {
		options.Module = "."
	}

	options.Type = strings.TrimSpace(options.Type)
	options.Package = strings.TrimSpace(options.Package)
	options.KeyNamer = strings.ToLower(strings.TrimSpace(options.KeyNamer))
	if options.KeyNamer == "" {
		options.KeyNamer = "none"
	}
	options.JSONSchemaVersion = strings.TrimSpace(options.JSONSchemaVersion)

	return options
}

// resolveJSONSchemaVersion returns helper JSON Schema dependency version.
func resolveJSONSchemaVersion(rawVersion string) (string, bool) {
	version := strings.TrimSpace(rawVersion)
	if version == "" {
		return schemaGeneratorJSONSchemaVersion, false
	}

	return version, true
}

// resolvedTarget stores normalized execution info for local or remote module.
type resolvedTarget struct {
	Source      string
	ModulePath  string
	ModuleDir   string
	ModuleVer   string
	PackagePath string
}

// resolveTarget resolves local/remote module source and validates options.
func resolveTarget(options Options) (resolvedTarget, error) {
	moduleInput := strings.TrimSpace(options.Module)
	if moduleInput == "" {
		return resolvedTarget{}, errors.New("module is required")
	}

	if localDir, ok := toExistingAbsolutePath(moduleInput); ok {
		modulePath, err := modulePathFromDir(localDir)
		if err != nil {
			return resolvedTarget{}, err
		}

		packagePath := strings.TrimSpace(options.Package)
		if packagePath == "" {
			packagePath = modulePath
		}

		return resolvedTarget{
			Source:      moduleSourceLocal,
			ModulePath:  modulePath,
			ModuleDir:   filepath.ToSlash(localDir),
			PackagePath: packagePath,
		}, nil
	}

	if modulePath, version, ok := parseRemoteModuleSpec(moduleInput); ok {
		if err := validateRemoteModulePath(modulePath); err != nil {
			return resolvedTarget{}, err
		}

		packagePath := strings.TrimSpace(options.Package)
		if packagePath == "" {
			packagePath = modulePath
		}

		return resolvedTarget{
			Source:      moduleSourceRemote,
			ModulePath:  modulePath,
			ModuleVer:   version,
			PackagePath: packagePath,
		}, nil
	}

	if looksLikeRemoteModulePath(moduleInput) {
		return resolvedTarget{}, fmt.Errorf(
			"remote module %q must include explicit version (for example: %s@v1.2.3 or %s@latest)",
			moduleInput,
			moduleInput,
			moduleInput,
		)
	}

	return resolvedTarget{}, fmt.Errorf(
		"module %q not found on disk and is not valid remote module reference",
		moduleInput,
	)
}

// parseRemoteModuleSpec parses "path@version" input.
func parseRemoteModuleSpec(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", false
	}

	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return "", "", false
	}

	modulePath := strings.TrimSpace(trimmed[:at])
	version := strings.TrimSpace(trimmed[at+1:])
	if modulePath == "" || version == "" {
		return "", "", false
	}

	return modulePath, version, true
}

// looksLikeRemoteModulePath reports whether value resembles module import path.
func looksLikeRemoteModulePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	if strings.ContainsAny(trimmed, " \t\r\n\\") {
		return false
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return false
	}

	return strings.Contains(parts[0], ".") || parts[0] == "gopkg.in"
}

// validateRemoteModulePath performs basic fast remote path validation.
func validateRemoteModulePath(modulePath string) error {
	if !looksLikeRemoteModulePath(modulePath) {
		return fmt.Errorf("invalid remote module path %q", modulePath)
	}

	if strings.Contains(modulePath, "//") {
		return fmt.Errorf("invalid remote module path %q: empty path segment", modulePath)
	}

	parts := strings.Split(modulePath, "/")
	for index := range parts {
		part := strings.TrimSpace(parts[index])
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid remote module path %q", modulePath)
		}

		if strings.Contains(part, "@") {
			return fmt.Errorf("invalid remote module path %q", modulePath)
		}
	}

	return nil
}

// modulePathFromDir resolves module path from local module directory.
func modulePathFromDir(moduleDir string) (string, error) {
	goModPath := filepath.Join(moduleDir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return "", fmt.Errorf("local module dir %q must contain go.mod", moduleDir)
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", goModPath, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "module "); ok {
			modulePath := strings.TrimSpace(after)
			modulePath = strings.Trim(modulePath, `"`)
			if modulePath == "" {
				return "", fmt.Errorf("parse %q: empty module path", goModPath)
			}

			return modulePath, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %q: %w", goModPath, err)
	}

	return "", fmt.Errorf("parse %q: module directive not found", goModPath)
}

// moduleDirFromHelperWorkspace resolves downloaded module dir for remote mode.
func moduleDirFromHelperWorkspace(helperDir string, modulePath string) (string, error) {
	command := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modulePath)
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

		return "", fmt.Errorf("resolve remote module dir %q: %s", modulePath, detail)
	}

	moduleDir := strings.TrimSpace(stdout.String())
	if moduleDir == "" {
		return "", fmt.Errorf("resolve remote module dir %q: empty result", modulePath)
	}

	absolutePath, err := filepath.Abs(moduleDir)
	if err != nil {
		return "", fmt.Errorf("resolve remote module dir %q: %w", modulePath, err)
	}

	return filepath.ToSlash(absolutePath), nil
}

// packageNameFromHelperWorkspace resolves package name for import path in helper.
func packageNameFromHelperWorkspace(helperDir, packagePath string) (string, error) {
	command := exec.Command("go", "list", "-f", "{{.Name}}", packagePath)
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

		return "", fmt.Errorf("load target package %q: %s", packagePath, detail)
	}

	name := strings.TrimSpace(stdout.String())
	if name == "" {
		return "", fmt.Errorf("load target package %q: empty package name", packagePath)
	}

	return name, nil
}

// createSchemaGeneratorDir creates temporary helper directory.
func createSchemaGeneratorDir() (string, error) {
	helperDir, err := os.MkdirTemp("", "schemadoc-mod2schema-")
	if err != nil {
		return "", fmt.Errorf("create temporary schema generator dir: %w", err)
	}

	return helperDir, nil
}

// writeSchemaGeneratorProgram stores temporary source code in helper
// directory.
func writeSchemaGeneratorProgram(helperDir, source string) error {
	helperPath := filepath.Join(helperDir, "main.go")
	if err := os.WriteFile(helperPath, []byte(source), 0o600); err != nil {
		return fmt.Errorf("write temporary schema generator: %w", err)
	}

	return nil
}

// initSchemaGeneratorWorkspace initializes temporary go module for schema
// generation.
func initSchemaGeneratorWorkspace(
	helperDir string,
	target resolvedTarget,
	jsonSchemaVersion string,
) error {
	helperModulePath := buildHelperModulePath(
		target.ModulePath,
		target.PackagePath,
	)
	if err := runGoCommand(helperDir, "mod", "init", helperModulePath); err != nil {
		return fmt.Errorf("init temporary module: %w", err)
	}

	requireVersion := "v0.0.0"
	switch target.Source {
	case moduleSourceLocal:
		requireVersion = localModulePlaceholderVersion(target.ModulePath)
	case moduleSourceRemote:
		requireVersion = target.ModuleVer
	}

	requireArg := "-require=" + target.ModulePath + "@" + requireVersion
	if err := runGoCommand(helperDir, "mod", "edit", requireArg); err != nil {
		return fmt.Errorf("require target module %q: %w", target.ModulePath, err)
	}

	if target.Source == moduleSourceLocal {
		replaceArg := "-replace=" + target.ModulePath + "=" + target.ModuleDir
		if err := runGoCommand(helperDir, "mod", "edit", replaceArg); err != nil {
			return fmt.Errorf("replace target module %q: %w", target.ModulePath, err)
		}

		if err := applySourceModuleReplaces(
			helperDir,
			target.ModuleDir,
			target.ModulePath,
		); err != nil {
			return err
		}
	}

	requireJSONSchemaArg := "-require=" +
		schemaGeneratorJSONSchemaModule + "@" + strings.TrimSpace(jsonSchemaVersion)
	if err := runGoCommand(
		helperDir,
		"mod",
		"edit",
		requireJSONSchemaArg,
	); err != nil {
		return fmt.Errorf(
			"require helper module %q: %w",
			schemaGeneratorJSONSchemaModule,
			err,
		)
	}

	return nil
}

// localModulePlaceholderVersion returns a Go-compatible local module version.
func localModulePlaceholderVersion(modulePath string) string {
	const defaultVersion = "v0.0.0"

	trimmedModulePath := strings.TrimSpace(modulePath)
	lastSlash := strings.LastIndex(trimmedModulePath, "/")
	if lastSlash < 0 {
		return defaultVersion
	}

	majorText, ok := strings.CutPrefix(trimmedModulePath[lastSlash+1:], "v")
	if !ok {
		return defaultVersion
	}

	major, err := strconv.Atoi(majorText)
	if err != nil || major < 2 || strconv.Itoa(major) != majorText {
		return defaultVersion
	}

	return fmt.Sprintf("v%d.0.0", major)
}

// applySourceModuleReplaces copies replace directives from source module go.mod.
func applySourceModuleReplaces(
	helperDir,
	moduleRootPath,
	targetModulePath string,
) error {
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
		replaceArg, ok, buildErr := buildReplaceEditArg(
			item,
			moduleRootPath,
			targetModulePath,
		)
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

// resolveActiveReplaceTarget resolves replacement target from go list module
// item.
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
func isLikelyLocalModuleDir(
	dir,
	modCachePath,
	moduleRootAbs,
	moduleParent string,
) bool {
	absoluteDir, ok := toExistingAbsolutePath(dir)
	if !ok {
		return false
	}

	if modCachePath != "" &&
		strings.HasPrefix(
			strings.ToLower(absoluteDir),
			strings.ToLower(modCachePath),
		) {
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

// buildReplaceEditArg converts one source replace into go mod edit -replace arg.
func buildReplaceEditArg(
	item goModEditReplace,
	moduleRootPath string,
	targetModulePath string,
) (string, bool, error) {
	oldPath := strings.TrimSpace(item.Old.Path)
	if oldPath == "" {
		return "", false, nil
	}

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

// isRelativeOrAbsolutePath reports whether replace target looks like filesystem
// path.
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

// runSchemaGeneratorProgram executes temporary schema generator.
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

// prepareSchemaGeneratorDependencies resolves helper module dependencies.
func prepareSchemaGeneratorDependencies(helperDir string) error {
	if err := runGoCommand(helperDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy helper module: %w", err)
	}

	return nil
}

// buildHelperModulePath returns temporary helper module path.
//
// When target package is under an internal path, helper module path is placed
// under the same parent import prefix to satisfy Go internal import rules.
func buildHelperModulePath(modulePath string, packagePath string) string {
	packagePath = strings.TrimSuffix(strings.TrimSpace(packagePath), "/")
	if packagePath != "" {
		internalPrefix, _, hasInternal := strings.Cut(packagePath, "/internal/")
		if hasInternal && strings.TrimSpace(internalPrefix) != "" {
			return internalPrefix + "/schemadoc_modschema_helper"
		}
	}

	modulePath = strings.TrimSuffix(strings.TrimSpace(modulePath), "/")
	if modulePath == "" {
		return "schemadoc_modschema_helper"
	}

	return modulePath + "/schemadoc_modschema_helper"
}

// ensureGoToolchain validates Go availability for mod2schema/mod2doc flows.
func ensureGoToolchain() error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go toolchain not found in PATH; mod2schema and mod2doc require installed Go")
	}

	return nil
}

// ensureGoVersionAtLeast validates active Go version for helper dependency.
func ensureGoVersionAtLeast(minVersion string) error {
	currentVersion, err := currentGoToolchainVersion()
	if err != nil {
		return err
	}

	currentMajor, currentMinor, err := parseGoMajorMinor(currentVersion)
	if err != nil {
		return fmt.Errorf("read go toolchain version: %w", err)
	}

	minMajor, minMinor, err := parseGoMajorMinor(minVersion)
	if err != nil {
		return fmt.Errorf("invalid minimum go version %q: %w", minVersion, err)
	}

	if currentMajor > minMajor || currentMajor == minMajor && currentMinor >= minMinor {
		return nil
	}

	return fmt.Errorf(
		"go toolchain %q is too old for %s@%s; require go >= %s or pass --jsonschema-version to override helper dependency version",
		currentVersion,
		schemaGeneratorJSONSchemaModule,
		schemaGeneratorJSONSchemaVersion,
		minVersion,
	)
}

// currentGoToolchainVersion returns active local Go toolchain version string.
func currentGoToolchainVersion() (string, error) {
	tmpDir, err := os.MkdirTemp("", "schemadoc-go-version-")
	if err != nil {
		return "", fmt.Errorf("create temporary dir for go version probe: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	command := exec.Command("go", "env", "GOVERSION")
	command.Dir = tmpDir

	// Force local toolchain to avoid auto-switch by unrelated go.mod/go.work.
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}

		return "", fmt.Errorf("read go env GOVERSION: %s", detail)
	}

	version := strings.TrimSpace(stdout.String())
	if version == "" {
		return "", errors.New("read go env GOVERSION: empty output")
	}

	return version, nil
}

// parseGoMajorMinor extracts numeric major/minor from Go version string.
func parseGoMajorMinor(version string) (int, int, error) {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "go")
	if trimmed == "" {
		return 0, 0, errors.New("empty version string")
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unsupported version format %q", version)
	}

	major, err := parseLeadingInt(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse major version from %q: %w", version, err)
	}

	minor, err := parseLeadingInt(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse minor version from %q: %w", version, err)
	}

	return major, minor, nil
}

// parseLeadingInt returns leading decimal integer from input segment.
func parseLeadingInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty numeric segment")
	}

	end := 0
	for end < len(value) {
		if value[end] < '0' || value[end] > '9' {
			break
		}

		end++
	}

	if end == 0 {
		return 0, fmt.Errorf("no leading digits in %q", value)
	}

	parsed, err := strconv.Atoi(value[:end])
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

// runGoCommand executes one Go command in selected directory.
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
