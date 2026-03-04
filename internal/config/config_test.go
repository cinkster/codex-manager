package config

import "testing"

func TestParseHTMLBucketFlag(t *testing.T) {
	cfg, err := Parse([]string{"-hb"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cfg.UseHTMLBucket {
		t.Fatalf("expected UseHTMLBucket=true")
	}
}
