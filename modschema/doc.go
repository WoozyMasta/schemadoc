// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package modschema generates JSON Schema from Go types
by running a temporary reflection helper with the Go toolchain.
The target package must be importable;
types in package main are not supported.

Use [Generate] for one request or [NewGenerator] to reuse prepared helpers.
Call [Generator.Close] to release a reusable session.
Context-aware variants support cancellation of setup, execution,
and waiting for a busy generator.

Generate only from trusted modules:
target packages and their transitive imports execute initialization code
with the caller's permissions, without a sandbox.
Go module checksums verify integrity, not trust.
*/
package modschema
