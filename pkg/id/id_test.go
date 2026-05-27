package id

import (
	"testing"
)

func TestNew_ProducesValidUUID(t *testing.T) {
	a := New()
	b := New()
	if a == "" || b == "" {
		t.Fatalf("empty id")
	}
	if a == b {
		t.Fatalf("ids must differ: %s == %s", a, b)
	}
	if err := Validate(a); err != nil {
		t.Fatalf("Validate failed for generated: %v", err)
	}
}

func TestValidate_AcceptsCanonical(t *testing.T) {
	if err := Validate("11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("canonical uuid rejected: %v", err)
	}
}

func TestValidate_RejectsGarbage(t *testing.T) {
	cases := []string{"", "abc", "not-a-uuid"}
	for _, c := range cases {
		if err := Validate(c); err == nil {
			t.Errorf("Validate(%q) should fail", c)
		}
	}
}
