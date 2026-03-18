package policy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestLoadFromFile(t *testing.T) {
	p, err := LoadFromFile(testdataPath("policy-typical.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Version != 1 {
		t.Fatalf("expected version 1, got %d", p.Version)
	}
	if p.Network == nil {
		t.Fatal("expected network policy")
	}
	if len(p.Network.Allow) == 0 {
		t.Fatal("expected network allow list")
	}
	if p.Filesystem == nil {
		t.Fatal("expected filesystem policy")
	}
	if p.Process == nil {
		t.Fatal("expected process policy")
	}
}

func TestLoadFromBytes_MissingVersion(t *testing.T) {
	_, err := LoadFromBytes([]byte(`network: {allow: ["*"]}`))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestLoadFromBytes_UnsupportedVersion(t *testing.T) {
	_, err := LoadFromBytes([]byte(`version: 99`))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestLoadFromBytes_InvalidYAML(t *testing.T) {
	_, err := LoadFromBytes([]byte(`{{{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFromBytes_OptionalSections(t *testing.T) {
	p, err := LoadFromBytes([]byte(`version: 1`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Network != nil {
		t.Fatal("expected nil network")
	}
	if p.Filesystem != nil {
		t.Fatal("expected nil filesystem")
	}
	if p.Process != nil {
		t.Fatal("expected nil process")
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.Version != 1 {
		t.Fatalf("expected version 1, got %d", p.Version)
	}
}
