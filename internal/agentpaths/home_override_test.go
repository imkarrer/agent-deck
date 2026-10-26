package agentpaths

import (
	"os"
	"path/filepath"
	"testing"
)

// seedGlobalDeck writes the files a user's normal deck would have under both
// the XDG layout and the legacy ~/.agent-deck layout, so a test can prove that
// AGENT_DECK_HOME never falls back to either of them.
func seedGlobalDeck(t *testing.T, home string) {
	t.Helper()
	for _, dir := range []string{
		filepath.Join(home, ".config", AppDirName),
		filepath.Join(home, ".local", "share", AppDirName, "profiles"),
		filepath.Join(home, ".agent-deck", "profiles"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		filepath.Join(home, ".config", AppDirName, "config.toml"),
		filepath.Join(home, ".agent-deck", "config.toml"),
	} {
		if err := os.WriteFile(file, []byte("# global deck\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHomeOverride_RootsEveryDirectory(t *testing.T) {
	home := setupHome(t)
	seedGlobalDeck(t, home)
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "xdg-cache"))
	deck := filepath.Join(root, "deck")
	t.Setenv(HomeEnv, deck)

	for name, resolve := range map[string]func() (string, error){
		"ConfigDir": ConfigDir,
		"DataDir":   DataDir,
		"CacheDir":  CacheDir,
		"LegacyDir": LegacyDir,
	} {
		got, err := resolve()
		if err != nil {
			t.Fatalf("%s() error = %v", name, err)
		}
		if got != deck {
			t.Errorf("%s() = %q, want %q (AGENT_DECK_HOME)", name, got, deck)
		}
	}

	configPath, err := EffectiveConfigPath("config.toml")
	if err != nil {
		t.Fatalf("EffectiveConfigPath() error = %v", err)
	}
	if want := filepath.Join(deck, "config.toml"); configPath != want {
		t.Errorf("EffectiveConfigPath() = %q, want %q: the global deck's config must not leak in", configPath, want)
	}

	dataDir, err := EffectiveDataDir("profiles")
	if err != nil {
		t.Fatalf("EffectiveDataDir() error = %v", err)
	}
	if dataDir != deck {
		t.Errorf("EffectiveDataDir() = %q, want %q: the global deck's profiles must not leak in", dataDir, deck)
	}

	runtimeDir, err := ProfileRuntimeDir("work")
	if err != nil {
		t.Fatalf("ProfileRuntimeDir() error = %v", err)
	}
	if want := filepath.Join(deck, "runtime", "profiles", "work"); runtimeDir != want {
		t.Errorf("ProfileRuntimeDir() = %q, want %q", runtimeDir, want)
	}

	cachePath, err := CachePath("update-cache.json")
	if err != nil {
		t.Fatalf("CachePath() error = %v", err)
	}
	if want := filepath.Join(deck, "update-cache.json"); cachePath != want {
		t.Errorf("CachePath() = %q, want %q", cachePath, want)
	}
}

func TestHomeOverride_IgnoresEmptyAndRelativeValues(t *testing.T) {
	for _, value := range []string{"", "   ", "relative/deck"} {
		t.Run(value, func(t *testing.T) {
			home := setupHome(t)
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv(HomeEnv, value)

			if got, ok := HomeOverride(); ok {
				t.Fatalf("HomeOverride() = %q, true; want no override for %q", got, value)
			}
			configDir, err := ConfigDir()
			if err != nil {
				t.Fatalf("ConfigDir() error = %v", err)
			}
			if want := filepath.Join(home, ".config", AppDirName); configDir != want {
				t.Errorf("ConfigDir() = %q, want %q", configDir, want)
			}
		})
	}
}

func TestHomeOverride_CleansValue(t *testing.T) {
	setupHome(t)
	deck := t.TempDir()
	t.Setenv(HomeEnv, "  "+deck+"/./ ")

	got, ok := HomeOverride()
	if !ok || got != deck {
		t.Fatalf("HomeOverride() = %q, %v; want %q, true", got, ok, deck)
	}
}

func TestHomeOverride_RefusesRealHomeUnderTest(t *testing.T) {
	realHome := osRealHome(t)
	setupHome(t)
	t.Setenv(HomeEnv, filepath.Join(realHome, "deck"))

	if dir, err := ConfigDir(); err == nil {
		t.Fatalf("ConfigDir() = %q, nil; want the real-home refusal for an AGENT_DECK_HOME under %q", dir, realHome)
	}
}

func TestHomeOverride_MigrationHasNothingToDo(t *testing.T) {
	setupHome(t)
	deck := t.TempDir()
	t.Setenv(HomeEnv, deck)
	if err := os.MkdirAll(filepath.Join(deck, "profiles", "default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deck, "config.toml"), []byte("# deck\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, force := range []bool{false, true} {
		result, err := MigrateLegacyLayout(MigrationOptions{Force: force})
		if err != nil {
			t.Fatalf("MigrateLegacyLayout(force=%v) error = %v; the legacy and XDG roots are the same directory", force, err)
		}
		if n := len(result.Copied) + len(result.Conflicts) + len(result.Skipped); n != 0 {
			t.Fatalf("MigrateLegacyLayout(force=%v) reported %d items, want none: %+v", force, n, result)
		}
	}
}
