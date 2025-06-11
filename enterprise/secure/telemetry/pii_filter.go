package telemetry

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "gopkg.in/yaml.v3"
)

// PIISchema represents a simple list of JSON field paths that are considered PII.
// The YAML file should look like:
// fields:
//   - user.email
//   - device.id
// Nested paths are dot-separated.
// For full protobuf awareness a more advanced implementation would use
// protobuf descriptors, but for this first iteration a JSON view is enough.
type PIISchema struct {
    Fields []string `yaml:"fields"`
}

// PIIFilter holds the compiled list of paths.
// It is not concurrency-safe for writes, but Read-only operations are safe.
type PIIFilter struct {
    paths [][]string // tokenised paths
}

// NewPIIFilterFromFile loads the YAML schema and returns a filter.
func NewPIIFilterFromFile(path string) (*PIIFilter, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var schema PIISchema
    if err := yaml.Unmarshal(data, &schema); err != nil {
        return nil, fmt.Errorf("failed to parse schema: %w", err)
    }
    if len(schema.Fields) == 0 {
        return nil, fmt.Errorf("schema has no fields")
    }
    paths := make([][]string, 0, len(schema.Fields))
    for _, p := range schema.Fields {
        parts := strings.Split(p, ".")
        paths = append(paths, parts)
    }
    return &PIIFilter{paths: paths}, nil
}

// RedactInPlace walks through the supplied JSON-serialisable map and redacts
// any PII fields. It returns the number of fields redacted.
func (f *PIIFilter) RedactInPlace(obj map[string]any) int {
    redacted := 0
    for _, path := range f.paths {
        if redactPath(obj, path) {
            redacted++
        }
    }
    return redacted
}

// Helper that traverses the object following the provided path and sets
// the final element to "REDACTED". Returns true if something was redacted.
func redactPath(curr any, path []string) bool {
    if len(path) == 0 {
        return false
    }
    m, ok := curr.(map[string]any)
    if !ok {
        return false
    }
    key := path[0]
    if len(path) == 1 {
        if _, exists := m[key]; exists {
            m[key] = "REDACTED"
            return true
        }
        return false
    }
    next, exists := m[key]
    if !exists {
        return false
    }
    return redactPath(next, path[1:])
}

// MarshalWithRedaction is a convenience utility: it marshals the provided struct
// to JSON, redacts PII fields according to the filter, then re-marshals.
// It returns the redacted JSON bytes and how many fields were redacted.
func (f *PIIFilter) MarshalWithRedaction(v any) ([]byte, int, error) {
    raw, err := json.Marshal(v)
    if err != nil {
        return nil, 0, err
    }
    var obj map[string]any
    if err := json.Unmarshal(raw, &obj); err != nil {
        return nil, 0, err
    }
    count := f.RedactInPlace(obj)
    out, err := json.Marshal(obj)
    if err != nil {
        return nil, 0, err
    }
    return out, count, nil
}
