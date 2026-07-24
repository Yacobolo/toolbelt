package netsuite

import "testing"

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"SC/SCMA/SCA - SuiteCommerce Solutions": "sc-scma-sca-suitecommerce-solutions",
		"NetSuite 2026.1 Release Notes":         "netsuite-2026-1-release-notes",
		"  Café & Commerce  ":                   "cafe-commerce",
		"What's New":                            "whats-new",
		"///":                                   "",
	}
	for input, want := range tests {
		if got := slugify(input); got != want {
			t.Errorf("slugify(%q) = %q, want %q", input, got, want)
		}
	}
}
