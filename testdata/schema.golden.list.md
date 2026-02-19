# schema reference

* Source schema: `testdata/schema.fixture.json`
* Schema ID: `urn:fixture:schema`
* Schema draft: `https://json-schema.org/draft/2020-12/schema`
* Draft support: `supported (2020-12)`
* Root ref: `#/$defs/Config`

## Contents

* [Config](#config)
* [Settings](#settings)

## Config

Root configuration object.

Attributes:

* Type: `object`
* Properties: 2

### Config.name

Key: `name`

Project name.

Attributes:

* Type: `string`
* Required: yes
* Constraints: minLength=1

### Config.Settings

Key: `settings`

Configuration settings.

Attributes:

* Required: no
* Reference: `#/$defs/Settings`

## Settings

Attributes:

* Type: `object`
* Properties: 1

### Settings.mode

Key: `mode`

Path: `settings.mode`

Attributes:

* Type: `string`
* Required: no
* Default: `"safe"`
* Enum: `"safe"`, `"fast"`
* Examples: `"safe"`
