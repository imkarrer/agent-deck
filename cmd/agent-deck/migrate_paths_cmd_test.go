package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupMigratePathsCommandHome(t *testing.T) (home, legacy string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("AGENT_DECK_HOME", "")
	legacy = filepath.Join(home, ".agent-deck")
	return home, legacy
}

func TestMigratePathsCommand_DryRunReportsPlannedCopiesAndDoesNotCopy(t *testing.T) {
	home, legacy := setupMigratePathsCommandHome(t)
	if err := os.MkdirAll(filepath.Join(legacy, "profiles", "default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("theme = \"dark\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "profiles", "default", "state.db"), []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runMigratePaths([]string{"--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runMigratePaths exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Migrating legacy ~/.agent-deck paths to XDG layout",
		"would copy config config.toml",
		"would copy data profiles",
		"legacy directory left untouched: " + legacy,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "agent-deck", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create config destination, stat err=%v", err)
	}
}

func TestMigratePathsCommand_ConflictReportsForceHint(t *testing.T) {
	home, legacy := setupMigratePathsCommandHome(t)
	legacyConfig := filepath.Join(legacy, "config.toml")
	xdgConfig := filepath.Join(home, ".config", "agent-deck", "config.toml")
	if err := os.MkdirAll(filepath.Dir(legacyConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(xdgConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyConfig, []byte("legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgConfig, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runMigratePaths(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected conflict exit\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "conflict config config.toml already exists at "+xdgConfig) {
		t.Fatalf("conflict output missing destination:\n%s", combined)
	}
	if !strings.Contains(combined, "rerun with --force to merge into existing XDG locations") {
		t.Fatalf("conflict output missing force hint:\n%s", combined)
	}
}

func TestMigratePathsCommand_HelpExitsSuccess(t *testing.T) {
	setupMigratePathsCommandHome(t)

	var stdout, stderr bytes.Buffer
	code := runMigratePaths([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runMigratePaths --help exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: agent-deck migrate-paths") || stderr.Len() != 0 {
		t.Fatalf("help must print usage on stdout only:\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

// Under AGENT_DECK_HOME the legacy and XDG roots are one directory, so there
// is nothing to copy and no XDG root to pin.
func TestMigratePathsCommand_AgentDeckHomeHasNothingToMigrate(t *testing.T) {
	setupMigratePathsCommandHome(t)
	deck := filepath.Join(t.TempDir(), "deck")
	t.Setenv("AGENT_DECK_HOME", deck)
	if err := os.MkdirAll(filepath.Join(deck, "profiles", "default"), 0o700); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := runMigratePaths(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("runMigratePaths exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if want := "AGENT_DECK_HOME is set"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("output missing %q:\n%s", want, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(deck, "profiles", ".active-root")); !os.IsNotExist(err) {
		t.Fatalf("migrate-paths pinned an XDG root inside AGENT_DECK_HOME, stat err=%v", err)
	}
}
