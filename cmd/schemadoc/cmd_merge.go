// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invopop/yaml"
	"github.com/woozymasta/schemadoc/merge"
)

// Execute runs merge subcommand.
func (command *schemaMergeCommand) Execute(_ []string) error {
	return command.runner.runSchemaMerge(
		command.Args.Input,
		command.Args.Output,
		command.MergeFlags,
	)
}

// runSchemaMerge executes schema merge request and writes result to output.
func (runner *cliRunner) runSchemaMerge(
	inputPath string,
	outputPath string,
	flags schemaMergeFlags,
) error {
	runner.logf(
		"merge: input=%s output=%s check=%t inplace=%t",
		firstNonEmpty(strings.TrimSpace(inputPath), "-"),
		firstNonEmpty(strings.TrimSpace(outputPath), "-"),
		flags.Check,
		flags.InPlace,
	)
	plan, err := buildSchemaMergePlan(inputPath, outputPath, flags)
	if err != nil {
		return err
	}

	if err := runner.executeSchemaMergePlan(plan); err != nil {
		return err
	}

	runner.logf("merge: ok")
	return nil
}

// UnmarshalFlag parses one mapping value in form key=value.
func (value *schemaMergeMap) UnmarshalFlag(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("empty mapping value")
	}

	key, mappedValue, ok := strings.Cut(trimmed, "=")
	if !ok {
		return fmt.Errorf("invalid mapping %q: expected key=value", raw)
	}

	key = strings.TrimSpace(key)
	mappedValue = strings.TrimSpace(mappedValue)
	if key == "" {
		return fmt.Errorf("invalid mapping %q: empty key", raw)
	}

	if mappedValue == "" {
		return fmt.Errorf("invalid mapping %q: empty value", raw)
	}

	if *value == nil {
		*value = make(map[string]string)
	}

	(*value)[key] = mappedValue
	return nil
}

// schemaMergePlan describes resolved merge execution plan.
type schemaMergePlan struct {
	SourcePath           string
	TargetPath           string
	Format               string
	Actions              []merge.Action
	Check                bool
	InPlace              bool
	PruneUnreachableDefs bool
}

// buildSchemaMergePlan composes runtime merge plan from CLI args and flags.
func buildSchemaMergePlan(
	inputPath string,
	outputPath string,
	flags schemaMergeFlags,
) (schemaMergePlan, error) {
	plan := schemaMergePlan{
		SourcePath:           strings.TrimSpace(inputPath),
		TargetPath:           strings.TrimSpace(outputPath),
		Format:               strings.TrimSpace(flags.Format),
		Check:                flags.Check,
		InPlace:              flags.InPlace,
		PruneUnreachableDefs: flags.PruneUnreachableDefs,
		Actions:              make([]merge.Action, 0, len(flags.Replace)+len(flags.Merge)+len(flags.MergeDefs)),
	}

	if strings.TrimSpace(flags.ConfigPath) != "" {
		loaded, err := loadSchemaMergeConfig(flags.ConfigPath)
		if err != nil {
			return schemaMergePlan{}, err
		}

		plan.SourcePath = firstNonEmpty(plan.SourcePath, strings.TrimSpace(loaded.SourcePath))
		plan.TargetPath = firstNonEmpty(plan.TargetPath, strings.TrimSpace(loaded.TargetPath))
		plan.Format = firstNonEmpty(plan.Format, strings.TrimSpace(loaded.Format))
		plan.Check = plan.Check || loaded.Check
		plan.InPlace = plan.InPlace || loaded.InPlace
		plan.PruneUnreachableDefs = plan.PruneUnreachableDefs || loaded.PruneUnreachableDefs
		plan.Actions = append(plan.Actions, loaded.Actions...)
	}

	actionsFromFlags, err := mergeActionsFromFlags(flags)
	if err != nil {
		return schemaMergePlan{}, err
	}

	plan.Actions = append(plan.Actions, actionsFromFlags...)
	return plan, nil
}

// schemaMergeConfigFile describes merge config for merge subcommand.
type schemaMergeConfigFile struct {
	// SourcePath is base schema source path.
	SourcePath string `json:"source,omitempty" yaml:"source,omitempty"`

	// TargetPath is optional output path.
	TargetPath string `json:"target,omitempty" yaml:"target,omitempty"`

	// Format selects output format.
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// Actions lists merge operations.
	Actions []merge.Action `json:"actions,omitempty" yaml:"actions,omitempty"`

	// Check enables diff-only output mode.
	Check bool `json:"check,omitempty" yaml:"check,omitempty"`

	// InPlace writes output into source path.
	InPlace bool `json:"inplace,omitempty" yaml:"inplace,omitempty"`

	// PruneUnreachableDefs removes unreachable defs after merge.
	PruneUnreachableDefs bool `json:"prune_unreachable_defs,omitempty" yaml:"prune_unreachable_defs,omitempty"`
}

// loadSchemaMergeConfig decodes merge config file.
func loadSchemaMergeConfig(path string) (schemaMergeConfigFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return schemaMergeConfigFile{}, fmt.Errorf("read merge config %q: %w", path, err)
	}

	var cfg schemaMergeConfigFile
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".json":
		if err := json.Unmarshal(content, &cfg); err != nil {
			return schemaMergeConfigFile{}, fmt.Errorf(
				"decode merge config json %q: %w",
				path,
				err,
			)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return schemaMergeConfigFile{}, fmt.Errorf(
				"decode merge config yaml %q: %w",
				path,
				err,
			)
		}
	default:
		if err := json.Unmarshal(content, &cfg); err != nil {
			if yamlErr := yaml.Unmarshal(content, &cfg); yamlErr != nil {
				return schemaMergeConfigFile{}, fmt.Errorf(
					"decode merge config %q: unsupported format",
					path,
				)
			}
		}
	}

	return cfg, nil
}

// mergeActionsFromFlags builds actions from map-based CLI flags.
func mergeActionsFromFlags(flags schemaMergeFlags) ([]merge.Action, error) {
	actions := make([]merge.Action, 0, len(flags.Replace)+len(flags.Merge)+len(flags.MergeDefs))

	for target, source := range flags.Replace {
		action, err := parseActionMapping(merge.NodeOpReplace, target, source, flags)
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	for target, source := range flags.Merge {
		action, err := parseActionMapping(merge.NodeOpMerge, target, source, flags)
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	for target, source := range flags.MergeDefs {
		action, err := parseActionMapping(merge.NodeOpMergeDefs, target, source, flags)
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// parseActionMapping parses target=source mapping into merge action.
func parseActionMapping(
	nodeOp string,
	targetPointer string,
	rawSource string,
	flags schemaMergeFlags,
) (merge.Action, error) {
	sourcePath, sourcePointer, err := splitSourceRef(rawSource)
	if err != nil {
		return merge.Action{}, err
	}

	return merge.Action{
		Type:          nodeOp,
		SourcePath:    sourcePath,
		SourcePointer: sourcePointer,
		TargetPointer: strings.TrimSpace(targetPointer),
		ObjectOp:      strings.TrimSpace(flags.ObjectOp),
		ArrayOp:       strings.TrimSpace(flags.ArrayOp),
	}, nil
}

// splitSourceRef parses path[#/pointer] reference.
func splitSourceRef(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", errors.New("source reference is empty")
	}

	pathPart, pointerPart, ok := strings.Cut(trimmed, "#")
	pathPart = strings.TrimSpace(pathPart)
	if pathPart == "" {
		return "", "", errors.New("source reference path is empty")
	}

	if !ok {
		return pathPart, "", nil
	}

	pointerPart = strings.TrimSpace(pointerPart)
	if pointerPart == "" {
		return "", "", errors.New("source reference pointer is empty")
	}

	return pathPart, "/" + strings.TrimPrefix(pointerPart, "/"), nil
}

// executeSchemaMergePlan runs merge plan and writes/checks output.
func (runner *cliRunner) executeSchemaMergePlan(plan schemaMergePlan) error {
	sourcePath := strings.TrimSpace(plan.SourcePath)
	if sourcePath == "" {
		return merge.ErrSourcePathRequired
	}

	merged, err := merge.File(
		sourcePath,
		plan.Actions,
		merge.ApplyOptions{
			PruneUnreachableDefs: plan.PruneUnreachableDefs,
		},
	)
	if err != nil {
		return err
	}

	outputPath := resolveMergeOutputPath(plan.TargetPath, sourcePath, plan.InPlace)
	format := resolveMergeOutputFormat(outputPath, plan.Format)
	encoded, err := merge.Encode(merged, format)
	if err != nil {
		return err
	}

	if plan.Check {
		if strings.TrimSpace(outputPath) == "" {
			return errors.New("check mode requires output path")
		}

		current, readErr := os.ReadFile(outputPath)
		if readErr != nil {
			return fmt.Errorf("read existing output %q: %w", outputPath, readErr)
		}

		if !bytes.Equal(current, encoded) {
			return fmt.Errorf("output differs from %q", outputPath)
		}

		return nil
	}

	return writeBytes(runner.stdout, outputPath, encoded, "schema")
}

// resolveMergeOutputFormat resolves output format by flag and output extension.
func resolveMergeOutputFormat(outputPath, format string) string {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized != "" {
		return normalized
	}

	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(outputPath)))
	switch extension {
	case ".yaml", ".yml":
		return merge.FormatYAML
	default:
		return merge.FormatJSON
	}
}

// resolveMergeOutputPath resolves write path for merge command.
func resolveMergeOutputPath(targetPath, sourcePath string, inPlace bool) string {
	if target := strings.TrimSpace(targetPath); target != "" {
		return target
	}

	if inPlace {
		return strings.TrimSpace(sourcePath)
	}

	return ""
}
