package redact

import (
	"strings"
	"testing"
)

func TestRedactScrubsSecrets(t *testing.T) {
	in := "use token AKIAIOSFODNN7EXAMPLE and ghp_0123456789012345678901234567890123456 to auth"
	out, hits := Redact(in)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key not redacted: %q", out)
	}
	if strings.Contains(out, "ghp_0123456789012345678901234567890123456") {
		t.Errorf("GitHub token not redacted: %q", out)
	}
	if !strings.Contains(out, "[REDACTED:") {
		t.Errorf("expected redaction markers: %q", out)
	}
	if len(hits) < 2 {
		t.Errorf("hits = %v, want >= 2", hits)
	}
}

func TestRedactLeavesCleanTextAlone(t *testing.T) {
	in := "The AWS_ACCESS_KEY_ID env var name is fine; vault path secret/data/app too."
	out, hits := Redact(in)
	if out != in {
		t.Errorf("clean text was modified:\n in:  %q\n out: %q", in, out)
	}
	if len(hits) != 0 {
		t.Errorf("unexpected hits on clean text: %v", hits)
	}
}
