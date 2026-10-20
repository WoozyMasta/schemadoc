// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/mod/modfile"
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

// localGoVersion caches only successful local toolchain probes for this process.
var localGoVersion toolchainVersionCache

// toolchainVersionCache serializes probes and allows retries after failures.
type toolchainVersionCache struct {
	version string      // Empty until a probe succeeds.
	mu      contextGate // Protects version and allows cancelled waiters to return.
}

// Generate reflects JSON Schema for selected module/package/type.
func Generate(options Options) ([]byte, string, error) {
	return GenerateContext(context.Background(), options)
}

// GenerateContext reflects one type and cleans its workspace after completion or cancellation.
func GenerateContext(ctx context.Context, options Options) ([]byte, string, error) {
	generator := NewGenerator()
	data, source, err := generator.GenerateContext(ctx, options)

	return data, source, errors.Join(err, generator.Close())
}

// ensureImportableTargetPackage validates package importability before helper run.
func ensureImportableTargetPackage(
	ctx context.Context,
	helperDir string,
	target resolvedTarget,
) error {
	packageName, err := packageNameFromHelperWorkspace(
		ctx,
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
func preloadRemoteModule(ctx context.Context, helperDir string, target resolvedTarget) error {
	if target.Source != moduleSourceRemote {
		return nil
	}

	if strings.TrimSpace(target.ModuleVer) == "" {
		return errors.New("remote module version is required")
	}

	targetRef := target.ModulePath + "@" + target.ModuleVer
	if err := runGoCommand(ctx, helperDir, "mod", "download", targetRef); err != nil {
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
	options.RootID = strings.TrimSpace(options.RootID)
	if options.Comments == "" {
		options.Comments = CommentModeRaw
	}
	if options.TypeNamer == "" {
		options.TypeNamer = TypeNamerModePackage
	}

	return options
}

// validateTypeNamerMode rejects unsupported definition naming modes before helper I/O.
func validateTypeNamerMode(mode TypeNamerMode) error {
	switch mode {
	case TypeNamerModePackage, TypeNamerModeFull:
		return nil
	default:
		return fmt.Errorf("%w %q: expected package or full", ErrInvalidTypeNamerMode, mode)
	}
}

// validateCommentMode rejects unsupported modes before any helper I/O.
func validateCommentMode(mode CommentMode) error {
	switch mode {
	case CommentModeRaw, CommentModePlain:
		return nil
	default:
		return fmt.Errorf("%w %q: expected raw or plain", ErrInvalidCommentMode, mode)
	}
}

// validateRootID validates an optional canonical root schema identifier.
func validateRootID(rootID string) error {
	if rootID == "" {
		return nil
	}

	parsed, err := url.Parse(rootID)
	if err != nil ||
		!parsed.IsAbs() ||
		strings.Contains(rootID, "#") ||
		strings.IndexFunc(rootID, unicode.IsSpace) >= 0 {
		return fmt.Errorf(
			"%w %q: must be an absolute URI without a fragment",
			ErrInvalidRootID,
			rootID,
		)
	}

	return nil
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
		return "", fmt.Errorf("local module dir %q must contain go.mod: %w", moduleDir, err)
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", goModPath, err)
	}

	parsed, err := modfile.Parse(goModPath, content, nil)
	if err != nil {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidModuleFile, goModPath, err)
	}

	if parsed.Module == nil || parsed.Module.Mod.Path == "" {
		return "", fmt.Errorf("%w %q: module directive is required", ErrInvalidModuleFile, goModPath)
	}

	return parsed.Module.Mod.Path, nil
}

// moduleDirFromHelperWorkspace resolves downloaded module dir for remote mode.
func moduleDirFromHelperWorkspace(ctx context.Context, helperDir string, modulePath string) (string, error) {
	stdout, err := goCommandOutput(ctx, helperDir, nil, "list", "-m", "-f", "{{.Dir}}", modulePath)
	if err != nil {
		return "", fmt.Errorf("resolve remote module dir %q: %w", modulePath, err)
	}

	moduleDir := strings.TrimSpace(string(stdout))
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
func packageNameFromHelperWorkspace(ctx context.Context, helperDir, packagePath string) (string, error) {
	stdout, err := goCommandOutput(ctx, helperDir, nil, "list", "-f", "{{.Name}}", packagePath)
	if err != nil {
		return "", fmt.Errorf("load target package %q: %w", packagePath, err)
	}

	name := strings.TrimSpace(string(stdout))
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
	ctx context.Context,
	helperDir string,
	target resolvedTarget,
	jsonSchemaVersion string,
) error {
	helperModulePath := buildHelperModulePath(
		target.ModulePath,
		target.PackagePath,
	)
	if err := runGoCommand(ctx, helperDir, "mod", "init", helperModulePath); err != nil {
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
	if err := runGoCommand(ctx, helperDir, "mod", "edit", requireArg); err != nil {
		return fmt.Errorf("require target module %q: %w", target.ModulePath, err)
	}

	if target.Source == moduleSourceLocal {
		replaceArg := "-replace=" + target.ModulePath + "=" + target.ModuleDir
		if err := runGoCommand(ctx, helperDir, "mod", "edit", replaceArg); err != nil {
			return fmt.Errorf("replace target module %q: %w", target.ModulePath, err)
		}

		if err := applySourceModuleReplaces(
			ctx,
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
		ctx,
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
	ctx context.Context,
	helperDir,
	moduleRootPath,
	targetModulePath string,
) error {
	replaces, err := listSourceModuleReplaces(ctx, moduleRootPath)
	if err != nil {
		return err
	}

	activeReplaces, err := listActiveModuleReplaces(ctx, moduleRootPath)
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
		if err := runGoCommand(ctx, helperDir, "mod", "edit", replaceArg); err != nil {
			return fmt.Errorf("apply source module replace %q: %w", replaceArg, err)
		}
	}

	return nil
}

// listSourceModuleReplaces reads replace directives from source module go.mod.
func listSourceModuleReplaces(ctx context.Context, moduleRootPath string) ([]goModEditReplace, error) {
	stdout, err := goCommandOutput(ctx, moduleRootPath, nil, "mod", "edit", "-json")
	if err != nil {
		return nil, fmt.Errorf("read source module go.mod replaces: %w", err)
	}

	var payload goModEditJSON
	if err := json.Unmarshal(stdout, &payload); err != nil {
		return nil, fmt.Errorf("decode source module go.mod replaces: %w", err)
	}

	return payload.Replace, nil
}

// listActiveModuleReplaces reads active replacements from module graph.
func listActiveModuleReplaces(ctx context.Context, moduleRootPath string) ([]goModEditReplace, error) {
	modCachePath, err := goEnvValue(ctx, moduleRootPath, "GOMODCACHE")
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

	stdout, err := goCommandOutput(ctx, moduleRootPath, nil, "list", "-m", "-json", "all")
	if err != nil {
		return nil, fmt.Errorf("read active module replaces: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout))
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
func goEnvValue(ctx context.Context, moduleRootPath, name string) (string, error) {
	stdout, err := goCommandOutput(ctx, moduleRootPath, nil, "env", name)
	if err != nil {
		return "", fmt.Errorf("read go env %s: %w", name, err)
	}

	return strings.TrimSpace(string(stdout)), nil
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
func runSchemaGeneratorProgram(ctx context.Context, helperDir string) ([]byte, error) {
	// Keep Go's own build temporaries inside the owned workspace for cancellation cleanup.
	env := append(os.Environ(), "GOTMPDIR="+helperDir)
	stdout, err := goCommandOutput(ctx, helperDir, env, "run", ".")
	if err != nil {
		return nil, fmt.Errorf("run module schema generator: %w", err)
	}

	return stdout, nil
}

// prepareSchemaGeneratorDependencies resolves helper module dependencies.
func prepareSchemaGeneratorDependencies(ctx context.Context, helperDir string) error {
	if err := runGoCommand(ctx, helperDir, "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy helper module: %w", err)
	}

	return nil
}

// downloadSchemaGeneratorDependencies populates helper module sums before the
// target package is inspected, without pruning its declared requirements.
func downloadSchemaGeneratorDependencies(ctx context.Context, helperDir string) error {
	if err := runGoCommand(ctx, helperDir, "mod", "download"); err != nil {
		return fmt.Errorf("download helper module dependencies: %w", err)
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
func ensureGoVersionAtLeast(ctx context.Context, minVersion string) error {
	currentVersion, err := currentGoToolchainVersion(ctx)
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
func currentGoToolchainVersion(ctx context.Context) (string, error) {
	return localGoVersion.load(ctx, probeGoToolchainVersion)
}

// load caches successful probes without making transient errors permanent.
func (cache *toolchainVersionCache) load(ctx context.Context, probe func(context.Context) (string, error)) (string, error) {
	if err := cache.mu.lock(ctx); err != nil {
		return "", err
	}
	defer cache.mu.unlock()

	if cache.version != "" {
		return cache.version, nil
	}

	version, err := probe(ctx)
	if err != nil {
		return "", err
	}

	cache.version = version
	return version, nil
}

// probeGoToolchainVersion runs outside module/workspace selection without temp allocation.
func probeGoToolchainVersion(ctx context.Context) (string, error) {
	// Force local toolchain to avoid auto-switch by unrelated go.mod/go.work.
	env := append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GO111MODULE=off")
	stdout, err := goCommandOutput(ctx, os.TempDir(), env, "env", "GOVERSION")
	if err != nil {
		return "", fmt.Errorf("read go env GOVERSION: %w", err)
	}

	version := strings.TrimSpace(string(stdout))
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
func runGoCommand(ctx context.Context, dir string, args ...string) error {
	_, err := goCommandOutput(ctx, dir, nil, args...)
	return err
}
