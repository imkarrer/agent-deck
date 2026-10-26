package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/agentpaths"
)

// setupAgentDeckHome points AGENT_DECK_HOME at a fresh directory beside a
// global deck that already holds a populated legacy store, a populated XDG
// store and a config.toml in both layouts: the state a second deck on the
// same machine must never read.
func setupAgentDeckHome(t *testing.T) (deck string) {
	t.Helper()
	xdgRoot, legacyRoot := setupStoreRootEnv(t)
	seedProfileStore(t, legacyRoot, "default", 2)
	seedProfileStore(t, xdgRoot, "default", 3)
	xdgConfig := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "agent-deck")
	for _, dir := range []string{xdgConfig, legacyRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, UserConfigFileName), []byte("default_tool = \"global-deck\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	deck = filepath.Join(t.TempDir(), "deck")
	t.Setenv(agentpaths.HomeEnv, deck)
	ResetStoreRootSelection()
	ClearUserConfigCache()
	return deck
}

func TestAgentDeckHome_StoreRootIsTheHomeAndNotDivergent(t *testing.T) {
	for _, seeded := range []int{-1, 0, 1} {
		t.Run(map[int]string{-1: "fresh", 0: "empty-store", 1: "populated-store"}[seeded], func(t *testing.T) {
			deck := setupAgentDeckHome(t)
			if seeded >= 0 {
				seedProfileStore(t, deck, "default", seeded)
			}

			sel, err := SelectStoreRoot()
			if err != nil {
				t.Fatalf("SelectStoreRoot() error = %v", err)
			}
			if sel.Active != deck {
				t.Fatalf("SelectStoreRoot().Active = %q, want AGENT_DECK_HOME %q", sel.Active, deck)
			}
			if sel.Divergent || sel.Warning() != "" {
				t.Fatalf("one directory is not two roots: divergent=%v reason=%s warning=%q", sel.Divergent, sel.Reason, sel.Warning())
			}
		})
	}
}

func TestAgentDeckHome_StorageAndConfigStayInsideTheHome(t *testing.T) {
	deck := setupAgentDeckHome(t)

	storage, err := NewStorageWithProfile("default")
	if err != nil {
		t.Fatalf("NewStorageWithProfile() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	if want := filepath.Join(deck, ProfilesDirName, "default", "state.db"); storage.Path() != want {
		t.Fatalf("storage path = %q, want %q", storage.Path(), want)
	}
	instances, err := storage.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("a new deck under AGENT_DECK_HOME loaded %d sessions of the global deck", len(instances))
	}

	configPath, err := GetUserConfigPath()
	if err != nil {
		t.Fatalf("GetUserConfigPath() error = %v", err)
	}
	if want := filepath.Join(deck, UserConfigFileName); configPath != want {
		t.Fatalf("GetUserConfigPath() = %q, want %q", configPath, want)
	}
	if got := GetHooksDir(); got != filepath.Join(deck, "hooks") {
		t.Fatalf("GetHooksDir() = %q, want it under %q", got, deck)
	}
}
