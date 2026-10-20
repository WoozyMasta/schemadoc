// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import "errors"

// errMod2DocRequiresMod2Schema reports a missing source for build documentation.
var errMod2DocRequiresMod2Schema = errors.New("mod2doc requires mod2schema in the same build document")

// errDuplicateBuildDefaults rejects ambiguous global settings before execution.
var errDuplicateBuildDefaults = errors.New("config contains more than one defaults document")

// errMixedBuildDefaults prevents treating a defaults document as an executable job.
var errMixedBuildDefaults = errors.New("defaults document must contain only the defaults key")

// errInvalidBuildDefaults rejects null or scalar defaults.
var errInvalidBuildDefaults = errors.New("defaults must be an object")

// errBuildDocumentNotObject rejects scalar and list stream entries.
var errBuildDocumentNotObject = errors.New("config document must be an object")
