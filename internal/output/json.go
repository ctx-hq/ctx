package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSON prints data as formatted JSON to stdout.
func JSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// JSONCompact prints data as compact JSON to stdout.
func JSONCompact(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

// JSONError prints an error as JSON.
func JSONError(err error) {
	_, _ = fmt.Fprintf(os.Stdout, `{"error":%q}`+"\n", err.Error())
}
