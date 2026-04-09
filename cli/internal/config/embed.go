package config

import _ "embed"

// SchemaJSON is the JSON Schema for config.yaml, embedded at build time.
//
//go:embed schema.json
var SchemaJSON []byte
