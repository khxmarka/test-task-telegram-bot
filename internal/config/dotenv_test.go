package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "TEST_DOTENV_FOO=bar\n# comment\nTEST_DOTENV_QUOTED=\"baz\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("TEST_DOTENV_FOO")
	_ = os.Unsetenv("TEST_DOTENV_QUOTED")
	LoadDotEnv(path)
	if os.Getenv("TEST_DOTENV_FOO") != "bar" {
		t.Fatalf("foo=%q", os.Getenv("TEST_DOTENV_FOO"))
	}
	if os.Getenv("TEST_DOTENV_QUOTED") != "baz" {
		t.Fatalf("quoted=%q", os.Getenv("TEST_DOTENV_QUOTED"))
	}
	// existing env not overwritten
	_ = os.Setenv("TEST_DOTENV_FOO", "keep")
	LoadDotEnv(path)
	if os.Getenv("TEST_DOTENV_FOO") != "keep" {
		t.Fatal("should not override")
	}
}
