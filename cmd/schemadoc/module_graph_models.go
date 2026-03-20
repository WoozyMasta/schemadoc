// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

// goModEditJSON is minimal payload from `go mod edit -json`.
type goModEditJSON struct {
	Replace []goModEditReplace `json:"Replace"`
}

// goModEditReplace stores one replace rule from `go mod edit -json`.
type goModEditReplace struct {
	Old goModEditModule `json:"Old"`
	New goModEditModule `json:"New"`
}

// goModEditModule stores module path/version tuple.
type goModEditModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
}

// goListModule is minimal payload from `go list -m -json all`.
type goListModule struct {
	Replace *goListModuleItem `json:"Replace"`
	Path    string            `json:"Path"`
	Version string            `json:"Version"`
	Dir     string            `json:"Dir"`
}

// goListModuleItem stores replacement module payload.
type goListModuleItem struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
}
