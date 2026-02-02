// Package cloudsavegame contains the main entry point logic and embedded resources.
package cloudsavegame

import "embed"

// RulesFS contains the embedded rules files.
//
//go:embed rules
var RulesFS embed.FS
