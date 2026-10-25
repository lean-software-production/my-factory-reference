package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// Each machine has a folder of its own in the factory's folder. An
// example's copy of the factory holds the machines' real folders (their
// AGENTS.md, machine.json and .gitignore) with a harness chosen for each
// machine and a login of its own that is a stand-in for yours.

func (w *world) registerMachineSteps(sc *godog.ScenarioContext) {
	sc.Step(`^every machine runs pi$`, w.everyMachineRunsPi)
	sc.Step(`^no harness is chosen for the validator$`, func() error { return w.setHarness("validator", "") })

	sc.Step(`^the (planner|doer|validator)'s AGENTS\.md says "([^"]*)"$`, w.saysInAGENTSmd)
	sc.Step(`^the folder above the target has an AGENTS\.md saying "([^"]*)"$`, func(words string) error {
		folder := filepath.Dir(w.targetDir())
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(folder, "AGENTS.md"), []byte(words+"\n"), 0o644)
	})
	sc.Step(`^the target's CODEWORD file says "([^"]*)"$`, func(word string) error {
		raw, err := os.ReadFile(filepath.Join(w.targetDir(), "CODEWORD"))
		if err != nil {
			return fmt.Errorf("the target has no CODEWORD file: %v\n%s", err, w.output)
		}
		if !strings.Contains(string(raw), word) {
			return fmt.Errorf("CODEWORD says %q, not %q", raw, word)
		}
		return nil
	})

	sc.Step(`^the doer has its own login$`, func() error { return w.giveLogin("doer") })
	sc.Step(`^the doer has no login$`, func() error { return os.Remove(w.loginPath("doer")) })
	sc.Step(`^there is an API key for the doer's provider$`, func() error {
		// openai-codex, the real machines' provider, has logins and no API
		// key, so the doer is moved to a provider that has one.
		if err := w.setModel("doer", "anthropic/claude-test"); err != nil {
			return err
		}
		w.env = append(w.env, "ANTHROPIC_API_KEY=a-key-for-anthropic")
		return nil
	})
	sc.Step(`^there is no API key for the doer's provider$`, func() error {
		return w.setModel("doer", "anthropic/claude-test") // and none is ever set
	})
	sc.Step(`^the doer signed in with its own login$`, func() error {
		signedIn, err := w.doerSignedIn()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(signedIn, "login ") || !strings.Contains(signedIn, `"owner":"doer"`) {
			return fmt.Errorf("the doer signed in with %q, not with its own login", signedIn)
		}
		return nil
	})
	sc.Step(`^the doer signed in with the API key$`, func() error {
		signedIn, err := w.doerSignedIn()
		if err != nil {
			return err
		}
		if signedIn != "api key" {
			return fmt.Errorf("the doer signed in with %q, not with an API key", signedIn)
		}
		return nil
	})
	sc.Step(`^the doer's login is ignored by Git$`, func() error {
		rel, err := filepath.Rel(w.repo, w.loginPath("doer"))
		if err != nil {
			return err
		}
		if _, err := git(w.repo, "check-ignore", "-q", rel); err != nil {
			return fmt.Errorf("Git does not ignore %s", rel)
		}
		return nil
	})
	sc.Step(`^it reports that the doer cannot sign in$`, func() error { return w.reports("the doer cannot sign in") })
}

// The factory's one assembly line is a folder in assembly-lines/, holding
// its graph and, in machines/, the machines on it.
func (w *world) linesDir() string { return filepath.Join(w.factoryDir(), "assembly-lines") }

func (w *world) machineDir(role string) string { return filepath.Join(w.lineDir, "machines", role) }

func (w *world) loginPath(role string) string { return filepath.Join(w.machineDir(role), "auth.json") }

var machineRoles = []string{"planner", "doer", "validator"}

// copyLine puts the real assembly line in the example's copy of the
// factory, its machines' folders included: only the files Git would keep,
// never a login. It chooses the double as every machine's harness, and
// gives each machine a login.
func (w *world) copyLine() error {
	real, err := filepath.Abs(filepath.Join("..", "assembly-lines"))
	if err != nil {
		return err
	}
	listed, err := git(filepath.Dir(real), "ls-files", "--cached", "--others", "--exclude-standard", "--", "assembly-lines")
	if err != nil {
		return err
	}
	for _, name := range strings.Split(listed, "\n") {
		to := filepath.Join(w.factoryDir(), name)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(filepath.Dir(real), name), to, 0o644); err != nil {
			return err
		}
	}
	lines, err := os.ReadDir(w.linesDir())
	if err != nil || len(lines) != 1 {
		return fmt.Errorf("expected the factory to have one assembly line in %s (%v)", w.linesDir(), err)
	}
	w.lineDir = filepath.Join(w.linesDir(), lines[0].Name())
	for _, role := range machineRoles {
		if err := w.setHarness(role, agentDouble); err != nil {
			return err
		}
		if err := w.giveLogin(role); err != nil {
			return err
		}
	}
	return nil
}

// config is a machine's machine.json.
func (w *world) config(role string) (map[string]any, error) {
	raw, err := os.ReadFile(filepath.Join(w.machineDir(role), "machine.json"))
	if err != nil {
		return nil, err
	}
	var config map[string]any
	return config, json.Unmarshal(raw, &config)
}

func (w *world) changeConfig(role string, change func(map[string]any)) error {
	config, err := w.config(role)
	if err != nil {
		return err
	}
	change(config)
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.machineDir(role), "machine.json"), append(raw, '\n'), 0o644)
}

// setHarness names the machine's harness in its machine.json; "" names none.
func (w *world) setHarness(role, harness string) error {
	return w.changeConfig(role, func(config map[string]any) {
		if harness == "" {
			delete(config, "harness")
		} else {
			config["harness"] = harness
		}
	})
}

func (w *world) setModel(role, model string) error {
	return w.changeConfig(role, func(config map[string]any) { config["model"] = model })
}

// giveLogin writes a stand-in for your login in the machine's folder, with
// an entry for the machine's provider that says whose copy it is.
func (w *world) giveLogin(role string) error {
	config, err := w.config(role)
	if err != nil {
		return err
	}
	provider, _, _ := strings.Cut(fmt.Sprint(config["model"]), "/")
	login, err := json.Marshal(map[string]any{provider: map[string]string{"type": "oauth", "owner": role}})
	if err != nil {
		return err
	}
	return os.WriteFile(w.loginPath(role), login, 0o600)
}

// saysInAGENTSmd adds the words to the machine's AGENTS.md, after what it
// says already.
func (w *world) saysInAGENTSmd(role, words string) error {
	path := filepath.Join(w.machineDir(role), "AGENTS.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, []byte("\n"+words+"\n")...), 0o644)
}

var lensLine = regexp.MustCompile(`(?m)^(Look at the work through one lens only: ).*$`)

// setLens changes the lens the validator's AGENTS.md names.
func (w *world) setLens(lens string) error {
	path := filepath.Join(w.machineDir("validator"), "AGENTS.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !lensLine.Match(raw) {
		return fmt.Errorf("the validator's AGENTS.md names no lens on a line like %q", lensLine)
	}
	return os.WriteFile(path, lensLine.ReplaceAll(raw, []byte("${1}"+lens+".")), 0o644)
}

// everyMachineRunsPi has each machine run the real pi, signed in with your
// own login, copied into the machine's folder as a student would.
func (w *world) everyMachineRunsPi() error {
	w.realPi = true
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	login, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "auth.json"))
	if err != nil {
		return fmt.Errorf("the real-agent examples need pi signed in: %v", err)
	}
	for _, role := range machineRoles {
		if err := w.setHarness(role, "pi"); err != nil {
			return err
		}
		if err := os.WriteFile(w.loginPath(role), login, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// doerSignedIn is how the doer signed in, the latest time it was called.
func (w *world) doerSignedIn() (string, error) {
	calls, err := w.callsTo("doer")
	if err != nil {
		return "", err
	}
	if len(calls) == 0 {
		return "", fmt.Errorf("the doer was never called:\n%s", w.output)
	}
	return calls[len(calls)-1].signedIn, nil
}
