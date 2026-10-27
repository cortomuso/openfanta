// Package rulesets embeds the built-in rule set files and their JSON Schema.
package rulesets

import "embed"

// FS holds every built-in rule set (*.yaml) and schema.json.
//
//go:embed *.yaml schema.json
var FS embed.FS
