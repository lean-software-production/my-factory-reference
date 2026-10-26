package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// repoWithChange is a repository with a file to commit.
func repoWithChange(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := prepareTarget(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A machine killed while Git was committing for it, or the Codespace put to
// sleep, can leave .git/index.lock behind with no Git to give it up. A
// factory started again must carry on, not fail every commit from then on.
func TestAStaleIndexLockDoesNotStopACommit(t *testing.T) {
	dir := repoWithChange(t)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitWork(dir, "Do the next task"); err != nil {
		t.Fatalf("the commit failed on a lock no Git holds: %v", err)
	}
	if _, err := os.Stat(lock); err == nil {
		t.Error("the stale index.lock was left behind")
	}
	if out, _ := git(dir, "log", "--format=%s"); !strings.Contains(out, "Do the next task") {
		t.Errorf("the work was not committed:\n%s", out)
	}
}

// A lock a running Git holds is not the factory's to remove: it waits for
// it, then fails saying why, leaving the lock as it was.
func TestALiveGitsIndexLockIsLeftAlone(t *testing.T) {
	dir := repoWithChange(t)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// A Git that stays until its input closes stands in for the one that
	// holds the lock.
	holder := exec.Command("git", "hash-object", "--stdin")
	stdin, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); holder.Wait() })
	defer func(wait time.Duration) { lockWait = wait }(lockWait)
	lockWait = 300 * time.Millisecond

	err = commitWork(dir, "Do the next task")
	if err == nil || !strings.Contains(err.Error(), "index.lock") {
		t.Errorf("the commit did not fail on a lock a Git holds: %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the lock a running Git holds was removed: %v", err)
	}
}

// A Git can give up its lock between the factory finding it and removing
// it as stale. The lock is gone, which is all the factory wanted.
func TestALockGivenUpAsItIsRemovedIsNoFailure(t *testing.T) {
	dir := repoWithChange(t)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func(was func() bool) { gitIsRunning = was }(gitIsRunning)
	gitIsRunning = func() bool {
		os.Remove(lock) // the Git finishes just as it is looked for
		return false
	}

	if err := clearIndexLock(dir); err != nil {
		t.Errorf("a lock that went as it was removed failed the commit: %v", err)
	}
}
