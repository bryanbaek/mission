package model

import "testing"

func TestModelPackageSmoke(t *testing.T) {
	t.Parallel()

	if RoleOwner == "" || RoleMember == "" {
		t.Fatal("expected role constants to be non-empty")
	}
}
