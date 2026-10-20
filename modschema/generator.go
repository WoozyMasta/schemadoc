// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Generator reuses prepared temporary helper modules until Close.
// Calls to Generate and Close are serialized. Its zero value is ready for use.
// Target module dependencies and replacement directives must remain unchanged during its lifetime.
// Close permanently ends the generator's session.
type Generator struct {
	workspaces map[workspaceKey]*schemaWorkspace // Owns prepared temporary helper modules.
	mu         contextGate                       // Serializes requests while allowing cancelled waiters to return.
	closed     bool                              // Prevents reuse after cleanup.
}

// workspaceKey isolates helper module semantics from request-level options.
type workspaceKey struct {
	target            resolvedTarget // Includes source, module path/directory/version and package.
	jsonSchemaVersion string         // Effective reflection dependency version.
}

// schemaWorkspace owns one prepared helper module and its resolved target.
type schemaWorkspace struct {
	dir     string         // Temporary directory owned by the generator.
	target  resolvedTarget // Includes the source directory resolved for remote modules.
	invalid bool           // Prevents reuse if cancellation cleanup could not remove this workspace.
}

// NewGenerator creates a reusable schema generation session.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate reflects one type, reusing module setup for compatible requests.
func (generator *Generator) Generate(options Options) ([]byte, string, error) {
	return generator.GenerateContext(context.Background(), options)
}

// GenerateContext reflects one type with cancellable setup, execution, and session waiting.
// Cancellation removes the affected workspace; other prepared workspaces remain reusable.
func (generator *Generator) GenerateContext(ctx context.Context, options Options) (schemaBytes []byte, sourcePath string, err error) {
	if err := generator.mu.lock(ctx); err != nil {
		return nil, "", err
	}
	defer generator.mu.unlock()

	if generator.closed {
		return nil, "", ErrGeneratorClosed
	}

	options = NormalizeOptions(options)

	if err := validateCommentMode(options.Comments); err != nil {
		return nil, "", err
	}

	if err := validateTypeNamerMode(options.TypeNamer); err != nil {
		return nil, "", err
	}

	if err := validateRootID(options.RootID); err != nil {
		return nil, "", err
	}

	target, err := resolveTarget(options)
	if err != nil {
		return nil, "", err
	}

	if target.Source == moduleSourceLocal {
		dir, err := filepath.EvalSymlinks(filepath.FromSlash(target.ModuleDir))
		if err != nil {
			return nil, "", fmt.Errorf("resolve canonical module directory: %w", err)
		}

		target.ModuleDir = filepath.ToSlash(dir)
	}

	version, customVersion := resolveJSONSchemaVersion(options.JSONSchemaVersion)
	key := workspaceKey{target: target, jsonSchemaVersion: version}
	workspace := generator.workspaces[key]
	defer func() {
		if ctx.Err() == nil {
			return
		}

		schemaBytes, sourcePath = nil, ""
		err = errors.Join(err, ctx.Err())

		if workspace != nil {
			err = errors.Join(err, generator.discardWorkspace(key, workspace))
		}
	}()

	if workspace != nil && workspace.invalid {
		if err := generator.discardWorkspace(key, workspace); err != nil {
			return nil, "", err
		}

		workspace = nil
	}

	if workspace == nil {
		if err := ensureGoToolchain(); err != nil {
			return nil, "", err
		}

		if !customVersion {
			if err := ensureGoVersionAtLeast(ctx, schemaGeneratorMinGoVersion); err != nil {
				return nil, "", err
			}
		}

		workspace, err = prepareSchemaWorkspace(ctx, target, version, options)
		if err != nil {
			return nil, "", err
		}

		if generator.workspaces == nil {
			generator.workspaces = make(map[workspaceKey]*schemaWorkspace)
		}

		generator.workspaces[key] = workspace
	} else if err := workspace.writeProgram(options); err != nil {
		return nil, "", err
	}

	data, err := runSchemaGeneratorProgram(ctx, workspace.dir)
	if err != nil {
		return nil, "", err
	}

	source := fmt.Sprintf("module:%s.%s", target.PackagePath, options.Type)
	return data, source, nil
}

// Close removes all owned workspaces. Repeated calls are safe;
// failed removals are retried on the next call, even though no further generation is allowed.
func (generator *Generator) Close() error {
	if err := generator.mu.lock(context.Background()); err != nil {
		return err
	}
	defer generator.mu.unlock()
	generator.closed = true
	var cleanupErrors []error

	for key, workspace := range generator.workspaces {
		if err := os.RemoveAll(workspace.dir); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove schema workspace %q: %w", workspace.dir, err))
			continue
		}

		delete(generator.workspaces, key)
	}

	return errors.Join(cleanupErrors...)
}

// discardWorkspace keeps failed removals owned but ineligible for later reflection.
func (generator *Generator) discardWorkspace(key workspaceKey, workspace *schemaWorkspace) error {
	workspace.invalid = true

	if err := os.RemoveAll(workspace.dir); err != nil {
		return fmt.Errorf("remove cancelled schema workspace %q: %w", workspace.dir, err)
	}

	delete(generator.workspaces, key)
	return nil
}

// prepareSchemaWorkspace completes dependency setup before transferring ownership.
func prepareSchemaWorkspace(ctx context.Context, target resolvedTarget, version string, options Options) (_ *schemaWorkspace, err error) {
	dir, err := createSchemaGeneratorDir()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("remove incomplete schema workspace %q: %w", dir, cleanupErr))
			}
		}
	}()

	if err := initSchemaGeneratorWorkspace(ctx, dir, target, version); err != nil {
		return nil, err
	}

	if err := downloadSchemaGeneratorDependencies(ctx, dir); err != nil {
		return nil, err
	}

	if err := ensureImportableTargetPackage(ctx, dir, target); err != nil {
		return nil, err
	}

	if target.Source == moduleSourceRemote {
		if err := preloadRemoteModule(ctx, dir, target); err != nil {
			return nil, err
		}

		target.ModuleDir, err = moduleDirFromHelperWorkspace(ctx, dir, target.ModulePath)
		if err != nil {
			return nil, err
		}
	}

	workspace := &schemaWorkspace{dir: dir, target: target}

	if err := workspace.writeProgram(options); err != nil {
		return nil, err
	}

	if err := prepareSchemaGeneratorDependencies(ctx, dir); err != nil {
		return nil, err
	}

	return workspace, nil
}

// writeProgram replaces request-specific source while helper imports stay fixed.
func (workspace *schemaWorkspace) writeProgram(options Options) error {
	source, err := renderProgramSource(templateData{
		PackagePath: workspace.target.PackagePath,
		TypeName:    options.Type,
		ModulePath:  workspace.target.ModulePath,
		ModuleDir:   workspace.target.ModuleDir,
		KeyNamer:    options.KeyNamer,
		RootID:      options.RootID,
		Comments:    options.Comments,
		TypeNamer:   options.TypeNamer,
	})
	if err != nil {
		return err
	}

	return writeSchemaGeneratorProgram(workspace.dir, source)
}
