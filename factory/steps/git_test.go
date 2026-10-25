package steps

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// git runs git in dir and returns its trimmed output.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// commitAll commits everything under the given paths of dir's repository.
func commitAll(dir, message string, paths ...string) error {
	if _, err := git(dir, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return err
	}
	_, err := git(dir, append([]string{"commit", "-q", "-m", message, "--"}, paths...)...)
	return err
}

// files maps each file under a folder, by its path there, to a hash of its
// contents. Git's own folder is left out.
type files map[string]string

func snapshot(dir string) (files, error) {
	snap := files{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) && path == dir {
			return filepath.SkipAll
		}
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		snap[rel] = fmt.Sprintf("%x", sha256.Sum256(raw))
		return nil
	})
	return snap, err
}

// diff says how two snapshots differ, or "" if they don't.
func (before files) diff(after files) string {
	var changes []string
	for _, path := range slices.Sorted(maps.Keys(before)) {
		if now, ok := after[path]; !ok {
			changes = append(changes, "removed "+path)
		} else if now != before[path] {
			changes = append(changes, "changed "+path)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[path]; !ok {
			changes = append(changes, "added "+path)
		}
	}
	return strings.Join(changes, "\n")
}

// repoState is the test repository apart from its targets: its files, and
// what is staged and unstaged in them.
type repoState struct {
	files  files
	status string
	staged string
}

// stateOutside records the repository, leaving out the given targets.
func stateOutside(repo string, targets []string) (*repoState, error) {
	all, err := snapshot(repo)
	if err != nil {
		return nil, err
	}
	state := &repoState{files: files{}}
	for path, hash := range all {
		if !underAny(path, targets) {
			state.files[path] = hash
		}
	}
	exclude := []string{"--", "."}
	for _, target := range targets {
		exclude = append(exclude, ":(exclude,literal)"+target)
	}
	if state.status, err = git(repo, append([]string{"status", "--porcelain", "-uall"}, exclude...)...); err != nil {
		return nil, err
	}
	if state.staged, err = git(repo, append([]string{"diff", "--cached"}, exclude...)...); err != nil {
		return nil, err
	}
	return state, nil
}

func underAny(path string, dirs []string) bool {
	for _, dir := range dirs {
		if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// newCommits lists the commits made in repo since the given one, oldest
// first, that touch path.
func newCommits(repo, since, path string) ([]string, error) {
	out, err := git(repo, "rev-list", "--reverse", since+"..HEAD", "--", path)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Fields(out), nil
}

// changedFiles lists the files a commit changed in the folder path, by
// their paths there.
func changedFiles(repo, commit, path string) ([]string, error) {
	out, err := git(repo, "diff-tree", "-r", "--no-commit-id", "--name-only", "-z", commit, "--", path)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, file := range strings.Split(out, "\x00") {
		if file == "" {
			continue
		}
		rel, err := filepath.Rel(path, file)
		if err != nil {
			return nil, err
		}
		changed = append(changed, rel)
	}
	return changed, nil
}

// productFiles leaves out the target's assembly lines, which hold its runs'
// plans: what is left is the work.
func productFiles(changed []string) []string {
	var product []string
	for _, file := range changed {
		if !underAny(file, []string{".assembly-lines"}) {
			product = append(product, file)
		}
	}
	return product
}
