// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import "errors"

// errMod2DocRequiresMod2Schema reports a missing source for build documentation.
var errMod2DocRequiresMod2Schema = errors.New("mod2doc requires mod2schema in the same build document")
