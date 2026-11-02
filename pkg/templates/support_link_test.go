package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Support links go through https://www.wasbot.app/wa, which redirects to the
// current support number (SUPPORT_WHATSAPP_NUMBER on wasbot-frontend). A
// hardcoded wa.me number would keep pointing at the old line after a switch.
func TestTemplatesUseSupportRedirectNotHardcodedNumber(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "wa.me/") {
				t.Errorf("%s:%d hardcodes a wa.me number; use https://www.wasbot.app/wa?text=...", f, i+1)
			}
		}
	}
}
