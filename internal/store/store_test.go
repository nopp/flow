package store

import (
	"path/filepath"
	"testing"
)

func TestStore_CreateRun_UpdateRunStatus_GetRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	id, err := st.CreateRun("my-app", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive run id, got %d", id)
	}

	if err := st.UpdateRunStatus(id, "running", "building..."); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateRunStatus(id, "success", "done"); err != nil {
		t.Fatal(err)
	}

	run, err := st.GetRun(id)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected run, got nil")
	}
	if run.AppID != "my-app" || run.Status != "success" || run.Log != "done" || run.CommitSHA != "abc123" {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestStore_ListRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.db")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, _ = st.CreateRun("app1", "")
	_, _ = st.CreateRun("app1", "")
	_, _ = st.CreateRun("app2", "")

	runs, err := st.ListRuns("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	runs, err = st.ListRuns("app1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for app1, got %d", len(runs))
	}
}

