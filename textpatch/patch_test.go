package textpatch_test

import (
	"errors"
	"testing"

	"github.com/bartdeboer/go-cfgr/textpatch"
)

func TestApplyUsesCodexStyleContext(t *testing.T) {
	original := "server:\n  host: localhost\n  port: 8080\n"
	patch := `*** Begin Patch
@@ server:
-  host: localhost
+  host: api.internal
*** End Patch`

	updated, err := textpatch.Apply(original, patch)
	if err != nil {
		t.Fatal(err)
	}
	if updated != "server:\n  host: api.internal\n  port: 8080\n" {
		t.Fatalf("updated contents:\n%s", updated)
	}
}

func TestApplyChangesMultipleHunksAtomically(t *testing.T) {
	original := "host: localhost\nport: 8080\nmode: development\n"
	patch := `*** Begin Patch
@@
-host: localhost
+host: api.internal
@@
-mode: development
+mode: production
*** End Patch`

	updated, err := textpatch.Apply(original, patch)
	if err != nil {
		t.Fatal(err)
	}
	if updated != "host: api.internal\nport: 8080\nmode: production\n" {
		t.Fatalf("updated contents:\n%s", updated)
	}
}

func TestApplyReturnsOriginalWhenAHunkDoesNotMatch(t *testing.T) {
	original := "enabled: false\n"
	patch := `*** Begin Patch
@@
-enabled: true
+enabled: false
*** End Patch`

	updated, err := textpatch.Apply(original, patch)
	if !errors.Is(err, textpatch.ErrNoMatch) {
		t.Fatalf("Apply() error = %v, want ErrNoMatch", err)
	}
	if updated != original {
		t.Fatalf("updated contents = %q, want original", updated)
	}
}

func TestApplyRejectsAmbiguousMatches(t *testing.T) {
	original := "enabled: false\nenabled: false\n"
	patch := `*** Begin Patch
@@
-enabled: false
+enabled: true
*** End Patch`

	updated, err := textpatch.Apply(original, patch)
	if !errors.Is(err, textpatch.ErrAmbiguous) {
		t.Fatalf("Apply() error = %v, want ErrAmbiguous", err)
	}
	if updated != original {
		t.Fatalf("updated contents = %q, want original", updated)
	}
}
