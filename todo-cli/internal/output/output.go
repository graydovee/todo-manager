package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatYAML   Format = "yaml"
	FormatPretty Format = "pretty" // deprecated alias for json
)

// Parse validates and normalizes a format string. "pretty" (and "yml")
// are accepted as aliases for "json" and "yaml" respectively.
func Parse(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatJSON, FormatPretty:
		return FormatJSON, nil
	case FormatYAML, "yml":
		return FormatYAML, nil
	}
	return "", fmt.Errorf("unknown output format %q (want yaml|json)", s)
}

func Write(w io.Writer, format Format, value any) error {
	var (
		data []byte
		err  error
	)
	switch format {
	case FormatYAML:
		data, err = marshalYAML(value)
	default: // json + pretty alias — both pretty now
		data, err = json.MarshalIndent(value, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// marshalYAML encodes with a 2-space indent to match the on-disk config format.
func marshalYAML(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
