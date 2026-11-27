package relay

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeleteProfilePreservesOtherProfilesAndSkipsEmptyFolders(t *testing.T) {
	root := t.TempDir()
	defaults := DefaultConfig("", "")
	first, second := defaults, defaults
	first.Project, first.Domain, first.Zone = "example-project", "play.example.com", "europe-west2-a"
	second.Project, second.Domain, second.Zone = "example-project", "web.example.com", "us-central1-f"
	for id, config := range map[string]Config{"a": first, "b": second} {
		if err := saveConfig(filepath.Join(root, id, "settings.json"), config); err != nil {
			t.Fatal(err)
		}
	}
	extra := filepath.Join(root, "a", "keep.txt")
	if err := os.WriteFile(extra, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	desktop, err := openDesktop(root, defaults)
	if err != nil {
		t.Fatal(err)
	}

	desktop.profiles["a"].setPhase("running", desktop.profiles["a"].GetState().Deadline, nil)
	if err := desktop.DeleteProfile("a"); err == nil {
		t.Fatal("deleted an active relay")
	}
	desktop.profiles["a"].setPhase("idle", desktop.profiles["a"].GetState().Deadline, nil)
	if err := desktop.DeleteProfile("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("deletion affected unrelated profile files: %v", err)
	}
	if _, err := os.Stat(desktop.path("a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted profile settings still exist: %v", err)
	}
	reopened, err := openDesktop(root, defaults)
	if err != nil {
		t.Fatal(err)
	}
	state := reopened.GetState()
	if len(state.Profiles) != 1 || state.ActiveProfile != "b" || !reflect.DeepEqual(state.Config, second) {
		t.Fatalf("reopened profiles = %+v", state)
	}

	if err := reopened.DeleteProfile("b"); err != nil {
		t.Fatal(err)
	}
	state = reopened.GetState()
	if len(state.Profiles) != 1 || !reflect.DeepEqual(state.Config, defaults) {
		t.Fatalf("reset profiles = %+v", state)
	}
}
