package cmd

import (
	"encoding/json"
	"fmt"
)

// printJSON marshals v as indented JSON and prints it to stdout.
func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
