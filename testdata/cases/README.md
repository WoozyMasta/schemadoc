# Schema test corpus

`manifest.json` is the registry for schema cases used by the staged
implementation plan. Each entry keeps the schema path, dialect, stable error
metadata, and future JSON/YAML or rendering expectations in one place.

The current cases are characterization fixtures. They make known regressions
visible without requiring fixes from later stages. Tests that assert generated
values, structured errors, or rendered output should activate the relevant
manifest entries when their implementation stage is complete.

## Layout

* `positive/` contains focused schema semantics and rendering inputs.
* `negative/` contains unresolved, unsupported, or unsatisfiable inputs.
* `../integration/kitchen-sink/` contains broad valid coverage.
* `../integration/adversarial/` contains unusual keys and annotation values.

Existing generated goldens remain under `../generated/` and are covered by
the pre-existing golden tests.
