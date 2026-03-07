<!-- Automatically generated file, do not modify! -->

# schema reference

* Source file: [`testdata/schema.fixture.json`](testdata/schema.fixture.json)
* Schema identifier: `urn:fixture:schema`
* JSON Schema version: `https://json-schema.org/draft/2020-12/schema`
* Version support: `supported (2020-12)`
* Root reference: `#/$defs/Config`

## Contents

* [Config](#config)
  * [Settings](#settings)

## Config

Root configuration object.

| Attribute | Value |
| --- | --- |
| Type | `object` |
| Properties | 2 |

### Config.name

Key: `name`

Project name.

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | yes |
| Constraints | minLength=1 |

### Config.Settings

Key: `settings`

Configuration settings.

| Attribute | Value |
| --- | --- |
| Required | no |
| Reference | [`Settings`](#settings) (`#/$defs/Settings`) |

## Settings

| Attribute | Value |
| --- | --- |
| Type | `object` |
| Properties | 1 |

### Settings.mode

Key: `mode`

Path: [`settings`](#configsettings).`mode`

| Attribute | Value |
| --- | --- |
| Type | `string` |
| Required | no |
| Default | `safe` |
| Enum | `safe`, `fast` |
| Examples | `safe` |

<!-- Automatically generated file, do not modify! -->
