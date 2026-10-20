// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package merge provides a reusable JSON Schema merge runtime.

Core library API is file-agnostic and works with in-memory documents.
File-oriented execution is provided as a convenience adapter.

Use [Apply] for in-memory schemas,
[File] for file inputs, and [Encode] to serialize the result.
Actions use JSON Pointers to select source and target nodes.
See the executable examples for both workflows.
*/
package merge
