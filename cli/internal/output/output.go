package output

import (
	"encoding/json"
	"fmt"
)

// Format holds the value of the global --format flag ("text" or "json").
// It is bound by cmd/root.go and read by any package that needs to branch
// on output format, including native plugin commands.
var Format string

// PrintJSON marshals v as indented JSON and prints it to stdout.
func PrintJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
