package api

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateUUID checks that s is a valid UUID.
func validateUUID(s, field string) error {
	if s == "" {
		return fmt.Errorf("%s is required", field)
	}
	if !uuidPattern.MatchString(s) {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	return nil
}

// validateEnum checks that s is one of the allowed values.
func validateEnum(s, field string, allowed []string) error {
	if s == "" {
		return fmt.Errorf("%s is required", field)
	}
	for _, a := range allowed {
		if s == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of %v", field, allowed)
}

// validateNonEmpty checks that s is not empty.
func validateNonEmpty(s, field string) error {
	if s == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

// validateJSON checks that raw is valid JSON.
func validateJSON(raw json.RawMessage, field string) error {
	if len(raw) == 0 {
		return nil // optional
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("%s must be valid JSON: %v", field, err)
	}
	return nil
}
