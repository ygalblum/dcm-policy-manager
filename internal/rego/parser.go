package rego

import (
	"errors"
	"strings"
)

var (
	ErrEmptyPackageDeclaration = errors.New("empty package declaration")
	ErrNoPackageDeclaration    = errors.New("no package declaration found in Rego code")
)

// ExtractPackageName extracts the package name from Rego code.
// It parses the Rego code looking for a "package" declaration and returns
// the package name (which may be namespaced, e.g., "policies.my_policy").
//
// Returns an error if:
// - No package declaration is found
// - The package declaration is empty
func ExtractPackageName(regoCode string) (string, error) {
	for line := range strings.SplitSeq(regoCode, "\n") {
		// Trim leading/trailing whitespace
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Look for package declaration (must be followed by space or end of line)
		if trimmed == "package" {
			return "", ErrEmptyPackageDeclaration
		}

		if pkgName, ok :=strings.CutPrefix(trimmed, "package "); ok && pkgName != "" {
			// Strip trailing comments (e.g., "package test # this is a comment")
			if idx := strings.Index(pkgName, "#"); idx != -1 {
				pkgName = pkgName[:idx]
			}

			// Trim any remaining whitespace
			pkgName = strings.TrimSpace(pkgName)

			// Validate package name is not empty
			if pkgName == "" {
				return "", ErrEmptyPackageDeclaration
			}

			return pkgName, nil
		}
	}

	return "", ErrNoPackageDeclaration
}
