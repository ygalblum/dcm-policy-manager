package service

import (
	"fmt"
	"strings"
)

// ConstraintContext tracks immutable fields set by higher-priority policies
type ConstraintContext struct {
	immutableFields map[string]bool   // field paths that cannot be modified
	setBy           map[string]string // field path → policy ID that set it
}

// NewConstraintContext creates a new ConstraintContext
func NewConstraintContext() *ConstraintContext {
	return &ConstraintContext{
		immutableFields: make(map[string]bool),
		setBy:           make(map[string]string),
	}
}

// MarkImmutable marks a field path as immutable and tracks which policy set it
func (c *ConstraintContext) MarkImmutable(fieldPath, policyID string) {
	c.immutableFields[fieldPath] = true
	c.setBy[fieldPath] = policyID
}

// IsImmutable checks if a field path is immutable
func (c *ConstraintContext) IsImmutable(fieldPath string) bool {
	return c.immutableFields[fieldPath]
}

// GetSetBy returns the policy ID that set the field, or empty string if not set
func (c *ConstraintContext) GetSetBy(fieldPath string) string {
	return c.setBy[fieldPath]
}

// CheckViolations compares the original and modified specs to detect constraint violations
// Returns a list of violated field paths
func (c *ConstraintContext) CheckViolations(original, modified map[string]interface{}) []string {
	violations := []string{}
	c.checkViolationsRecursive("", original, modified, &violations)
	return violations
}

// checkViolationsRecursive recursively checks for constraint violations
func (c *ConstraintContext) checkViolationsRecursive(
	prefix string,
	original, modified map[string]interface{},
	violations *[]string,
) {
	for key, modifiedValue := range modified {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		originalValue, existedBefore := original[key]

		// Check if this field is immutable
		if c.IsImmutable(fieldPath) {
			// If the value has changed, it's a violation
			if !valuesEqual(originalValue, modifiedValue) {
				*violations = append(*violations, fieldPath)
			}
		}

		// Recursively check nested maps
		if modifiedMap, ok := modifiedValue.(map[string]interface{}); ok {
			if originalMap, ok := originalValue.(map[string]interface{}); ok {
				c.checkViolationsRecursive(fieldPath, originalMap, modifiedMap, violations)
			} else if existedBefore {
				// Original was not a map but modified is - this is a change
				// Check if the parent path is immutable
				if c.IsImmutable(fieldPath) {
					*violations = append(*violations, fieldPath)
				}
			}
		}
	}
}

// MarkChangedFields compares original and modified specs and marks all changed fields as immutable
func (c *ConstraintContext) MarkChangedFields(original, modified map[string]interface{}, policyID string) {
	c.markChangedFieldsRecursive("", original, modified, policyID)
}

// markChangedFieldsRecursive recursively marks changed fields as immutable
func (c *ConstraintContext) markChangedFieldsRecursive(
	prefix string,
	original, modified map[string]interface{},
	policyID string,
) {
	for key, modifiedValue := range modified {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		originalValue, existedBefore := original[key]

		// Check if this is a nested map
		if modifiedMap, ok := modifiedValue.(map[string]interface{}); ok {
			if originalMap, ok := originalValue.(map[string]interface{}); ok {
				// Both are maps - recurse
				c.markChangedFieldsRecursive(fieldPath, originalMap, modifiedMap, policyID)
			} else {
				// Original was not a map (or didn't exist), but modified is
				// Mark the entire field as immutable
				c.MarkImmutable(fieldPath, policyID)
			}
		} else {
			// Not a map - check if value changed
			if !existedBefore || !valuesEqual(originalValue, modifiedValue) {
				c.MarkImmutable(fieldPath, policyID)
			}
		}
	}
}

// valuesEqual compares two values for equality
func valuesEqual(a, b interface{}) bool {
	// Simple comparison using fmt.Sprintf for now
	// This handles primitives and basic types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// FormatViolationError creates a formatted error message for constraint violations
func FormatViolationError(violations []string, constraintCtx *ConstraintContext) string {
	if len(violations) == 0 {
		return ""
	}

	parts := make([]string, len(violations))
	for i, fieldPath := range violations {
		setBy := constraintCtx.GetSetBy(fieldPath)
		parts[i] = fmt.Sprintf("%s (set by %s)", fieldPath, setBy)
	}

	return "Constraint violations: " + strings.Join(parts, ", ")
}
