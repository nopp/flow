package store

import (
	"path/filepath"
	"testing"
)

func TestStore_CreateRun_UpdateRunStatus_GetRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := New("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	id, err := st.CreateRun("my-app", "abc123", "admin")
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
	if run.AppID != "my-app" || run.Status != "success" || run.Log != "done" || run.CommitSHA != "abc123" || run.TriggeredBy != "admin" {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestStore_ListRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.db")
	st, err := New("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, _ = st.CreateRun("app1", "", "admin")
	_, _ = st.CreateRun("app1", "", "admin")
	_, _ = st.CreateRun("app2", "", "alice")

	runs, err := st.ListRuns("", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	runs, err = st.ListRuns("app1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for app1, got %d", len(runs))
	}
}

func TestStore_UserGroupAppRelationships(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acl.db")
	st, err := New("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	devID, err := st.CreateGroup("dev")
	if err != nil {
		t.Fatal(err)
	}
	opsID, err := st.CreateGroup("ops")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := st.CreateUser("alice", "hash", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserGroups(userID, []int64{devID, opsID}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppGroups("app-a", []int64{devID}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppGroups("app-b", []int64{opsID}); err != nil {
		t.Fatal(err)
	}

	groups, err := st.UserGroupIDs(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	appIDs, err := st.AppIDsByUserGroupIDs(groups)
	if err != nil {
		t.Fatal(err)
	}
	if len(appIDs) != 2 {
		t.Fatalf("expected 2 apps, got %d (%v)", len(appIDs), appIDs)
	}
}

func TestStore_DeleteRunsByAppID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-runs.db")
	st, err := New("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, _ = st.CreateRun("app1", "", "admin")
	_, _ = st.CreateRun("app1", "", "admin")
	_, _ = st.CreateRun("app2", "", "alice")

	if err := st.DeleteRunsByAppID("app1"); err != nil {
		t.Fatal(err)
	}

	count1, err := st.CountRuns("app1")
	if err != nil {
		t.Fatal(err)
	}
	if count1 != 0 {
		t.Fatalf("expected 0 runs for app1, got %d", count1)
	}

	count2, err := st.CountRuns("app2")
	if err != nil {
		t.Fatal(err)
	}
	if count2 != 1 {
		t.Fatalf("expected 1 run for app2, got %d", count2)
	}
}
