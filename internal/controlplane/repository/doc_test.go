package repository

import "testing"

func TestRepositoryPackageSmoke(t *testing.T) {
	t.Parallel()

	if ErrNotFound == nil || ErrNotFound.Error() != "not found" {
		t.Fatalf("ErrNotFound = %v, want not found", ErrNotFound)
	}
}
