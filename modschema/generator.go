// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Generator reuses prepared temporary helper modules until Close.
// Calls to Generate and Close are serialized. Its zero value is ready for use.
// Target module dependencies and replacement directives must remain unchanged during its lifetime.
// Close permanently ends the generator's session.
type Generator struct {
	workspaces map[workspaceKey]*schemaWorkspace // Owns prepared temporary helper modules.
	mu         sync.Mutex                        // Serializes workspace writes and helper execution.
	closed     bool                              // Prevents reuse after cleanup.
}

// workspaceKey isolates helper module semantics from request-level options.
type workspaceKey struct {
	target            resolvedTarget // Includes source, module path/directory/version and package.
	jsonSchemaVersion string         // Effective reflection dependency version.
}

// schemaWorkspace owns one prepared helper module and its resolved target.
type schemaWorkspace struct {
	dir    string         // Temporary directory owned by the generator.
	target resolvedTarget // Includes the source directory resolved for remote modules.
}

// NewGenerator creates a reusable schema generation session.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate reflects one type, reusing module setup for compatible requests.
func (generator *Generator) Generate(options Options) ([]byte, string, error) {
	generator.mu.Lock()
	defer generator.mu.Unlock()

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

	if workspace == nil {
		if err := ensureGoToolchain(); err != nil {
			return nil, "", err
		}

		if !customVersion {
			if err := ensureGoVersionAtLeast(schemaGeneratorMinGoVersion); err != nil {
				return nil, "", err
			}
		}

		workspace, err = prepareSchemaWorkspace(target, version, options)
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

	data, err := runSchemaGeneratorProgram(workspace.dir)
	if err != nil {
		return nil, "", err
	}

	source := fmt.Sprintf("module:%s.%s", target.PackagePath, options.Type)
	return data, source, nil
}

// Close removes all owned workspaces. Repeated calls are safe;
// failed removals are retried on the next call, even though no further generation is allowed.
func (generator *Generator) Close() error {
	generator.mu.Lock()
	defer generator.mu.Unlock()
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

// prepareSchemaWorkspace completes dependency setup before transferring ownership.
func prepareSchemaWorkspace(target resolvedTarget, version string, options Options) (_ *schemaWorkspace, err error) {
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

	if err := initSchemaGeneratorWorkspace(dir, target, version); err != nil {
		return nil, err
	}

	if err := downloadSchemaGeneratorDependencies(dir); err != nil {
		return nil, err
	}

	if err := ensureImportableTargetPackage(dir, target); err != nil {
		return nil, err
	}

	if target.Source == moduleSourceRemote {
		if err := preloadRemoteModule(dir, target); err != nil {
			return nil, err
		}

		target.ModuleDir, err = moduleDirFromHelperWorkspace(dir, target.ModulePath)
		if err != nil {
			return nil, err
		}
	}

	workspace := &schemaWorkspace{dir: dir, target: target}

	if err := workspace.writeProgram(options); err != nil {
		return nil, err
	}

	if err := prepareSchemaGeneratorDependencies(dir); err != nil {
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
