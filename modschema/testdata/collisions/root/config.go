// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package collisionroot

import (
	a "github.com/acme/a/common"
	b "github.com/acme/b/common"
)

// Config includes distinct external types with identical package-base names.
type Config struct {
	First  *a.Config  `json:"first"`  // Uses a pointer to the first external type.
	Second b.Config   `json:"second"` // Uses a different external type with the same name.
	Again  []a.Config `json:"again"`  // Reuses the first external definition.
	Local  Local      `json:"local"`  // Retains a target-package definition name.
}

// Local is a target-package type whose name is stable in both modes.
type Local struct {
	Value bool `json:"value"` // Identifies the local definition.
}

// Single includes only one external package and has no default naming collision.
type Single struct {
	First a.Config `json:"first"` // Keeps the existing common_Config definition name.
}
