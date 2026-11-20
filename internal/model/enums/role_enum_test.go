package enums

import "testing"

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{ROLE_SUPER_ADMIN, true},
		{ROLE_SYSTEM_ADMIN, true},
		{ROLE_SECURITY_ADMIN, true},
		{ROLE_AUDIT_ADMIN, true},
		{"", false},
		{"admin", false},
		{"unknownRole", false},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if got := IsValidRole(tt.role); got != tt.want {
				t.Errorf("IsValidRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestAllRoles(t *testing.T) {
	roles := AllRoles()
	if len(roles) != 4 {
		t.Fatalf("AllRoles() len = %d, want 4", len(roles))
	}
	for _, r := range roles {
		if !IsValidRole(r) {
			t.Errorf("AllRoles() contains invalid role %q", r)
		}
	}
}
