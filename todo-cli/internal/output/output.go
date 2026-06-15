package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatPretty Format = "pretty"
)

func Write(w io.Writer, format Format, value any) error {
	var (
		data []byte
		err  error
	)
	switch format {
	case FormatPretty:
		data, err = json.MarshalIndent(value, "", "  ")
	default:
		data, err = json.Marshal(value)
	}
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
