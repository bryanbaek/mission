package controller

import "testing"

func TestControllerPackageSmoke(t *testing.T) {
	t.Parallel()

	if tokenPrefix != "mssn_" {
		t.Fatalf("tokenPrefix = %q, want mssn_", tokenPrefix)
	}
}
