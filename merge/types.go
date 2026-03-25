// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

const (
	// FormatJSON selects JSON output format.
	FormatJSON = "json"
	// FormatYAML selects YAML output format.
	FormatYAML = "yaml"
	// ArrayOpReplace replaces destination array with source array.
	ArrayOpReplace = "replace"
	// ArrayOpAppend appends source items to destination array.
	ArrayOpAppend = "append"
	// ArrayOpAppendUnique appends only missing source items.
	ArrayOpAppendUnique = "append-unique"
	// ObjectOpMerge recursively merges object fields.
	ObjectOpMerge = "merge"
	// ObjectOpReplace fully replaces destination object.
	ObjectOpReplace = "replace"
	// NodeOpReplace replaces target node.
	NodeOpReplace = "replace"
	// NodeOpMerge deep-merges target object node.
	NodeOpMerge = "merge"
	// NodeOpMergeDefs merges object fields into target object.
	NodeOpMergeDefs = "merge-defs"
)

// Action describes one merge operation.
type Action struct {
	// Type selects node operation:
	// replace, merge, or merge-defs.
	Type string

	// SourcePath is source schema file path.
	//
	// When Source is provided, SourcePath is optional.
	SourcePath string

	// Source is optional in-memory source document.
	//
	// When set, merge reads source node from this value and ignores SourcePath.
	Source any `json:"-" yaml:"-"`

	// SourcePointer points to source node in source schema.
	//
	// Empty value selects root node.
	SourcePointer string

	// TargetPointer points to destination node in target schema.
	//
	// Empty value selects root node.
	TargetPointer string

	// ObjectOp configures object merge mode.
	ObjectOp string

	// ArrayOp configures array merge mode.
	ArrayOp string
}

// SourceLoader loads source schema documents by path.
type SourceLoader interface {
	// Load returns decoded schema node for path.
	Load(path string) (any, error)
}

// ApplyOptions configure pure merge execution.
type ApplyOptions struct {
	// Loader loads source documents for actions that use SourcePath.
	//
	// When nil, FileLoader is used.
	Loader SourceLoader

	// PruneUnreachableDefs removes defs not reachable by internal refs.
	PruneUnreachableDefs bool
}
