// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import "strings"

// schemaDialect identifies the keyword semantics used by schema processing.
type schemaDialect string

const (
	schemaDialectUnknown          schemaDialect = ""
	schemaDialectDraft4Compatible schemaDialect = "draft4-compatible"
	schemaDialectDraft6           schemaDialect = "draft-06"
	schemaDialectDraft7           schemaDialect = "draft-07"
	schemaDialect201909           schemaDialect = "2019-09"
	schemaDialect202012           schemaDialect = "2020-12"
)

// draftAliases maps normalized aliases to one semantic dialect.
var draftAliases = map[string]schemaDialect{
	"draft-04": schemaDialectDraft4Compatible,
	"draft-05": schemaDialectDraft4Compatible,
	"draft-4":  schemaDialectDraft4Compatible,
	"draft-5":  schemaDialectDraft4Compatible,
	"draft4":   schemaDialectDraft4Compatible,
	"draft5":   schemaDialectDraft4Compatible,

	"draft-06": schemaDialectDraft6,
	"draft6":   schemaDialectDraft6,
	"draft-07": schemaDialectDraft7,
	"draft7":   schemaDialectDraft7,
	"2019-09":  schemaDialect201909,
	"2020-12":  schemaDialect202012,

	"https://json-schema.org/draft-04/schema":      schemaDialectDraft4Compatible,
	"http://json-schema.org/draft-04/schema":       schemaDialectDraft4Compatible,
	"https://json-schema.org/draft-05/schema":      schemaDialectDraft4Compatible,
	"http://json-schema.org/draft-05/schema":       schemaDialectDraft4Compatible,
	"https://json-schema.org/draft-06/schema":      schemaDialectDraft6,
	"http://json-schema.org/draft-06/schema":       schemaDialectDraft6,
	"https://json-schema.org/draft-07/schema":      schemaDialectDraft7,
	"http://json-schema.org/draft-07/schema":       schemaDialectDraft7,
	"https://json-schema.org/draft/2019-09/schema": schemaDialect201909,
	"http://json-schema.org/draft/2019-09/schema":  schemaDialect201909,
	"https://json-schema.org/draft/2020-12/schema": schemaDialect202012,
	"http://json-schema.org/draft/2020-12/schema":  schemaDialect202012,
}

// normalizeSchemaDialect converts a $schema value to one supported semantic dialect.
func normalizeSchemaDialect(raw string) schemaDialect {
	normalized := normalizeDraft(raw)
	if normalized == "" {
		return schemaDialectUnknown
	}

	return draftAliases[normalized]
}

// detectDraft normalizes and resolves raw $schema value into DraftInfo metadata.
func detectDraft(raw string) DraftInfo {
	normalized := normalizeDraft(raw)
	if normalized == "" {
		return DraftInfo{Raw: raw}
	}

	dialect := normalizeSchemaDialect(raw)
	if dialect == schemaDialectUnknown {
		return DraftInfo{Raw: raw, Canonical: normalized, Supported: false}
	}

	canonical := string(dialect)
	if normalized == "draft-05" || strings.HasSuffix(normalized, "/draft-05/schema") {
		canonical = "draft-05"
	}

	return DraftInfo{Raw: raw, Canonical: canonical, Supported: true}
}

// normalizeDraft normalizes draft strings for matching by lower-casing and trimming suffixes.
func normalizeDraft(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	for normalized != "" {
		trimmed := strings.TrimSuffix(strings.TrimSuffix(normalized, "#"), "/")
		if trimmed == normalized {
			break
		}
		normalized = trimmed
	}

	return normalized
}

// DetectDraft reports draft support info for a single $schema value.
func DetectDraft(schemaURI string) DraftInfo {
	return detectDraft(schemaURI)
}
