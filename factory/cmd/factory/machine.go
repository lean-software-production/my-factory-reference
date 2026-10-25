package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// A machine does one kind of job for the factory: the planner keeps the
// plan, the doer does the work, and the validator checks it. Each has a
// folder of its own, machines/<name>/ in the factory's folder, which is the
// configuration of the harness that runs it: its AGENTS.md, its
// machine.json, and its own copy of your login.
type machine struct {
	role    string
	dir     string // the machine's folder
	harness string // the command that runs it
	model   string // <provider>/<model>
}

// config is machine.json.
type config struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// machineRoles are the machines a run needs, in the order they first work.
var machineRoles = []string{"planner", "doer", "validator"}

// defaultHarness runs a machine whose machine.json names none.
const defaultHarness = "pi"

// loadMachines reads the three machines' folders from the factory's.
func loadMachines() (map[string]machine, error) {
	root, err := machinesFolder()
	if err != nil {
		return nil, err
	}
	machines := map[string]machine{}
	for _, role := range machineRoles {
		m, err := loadMachine(role, filepath.Join(root, role))
		if err != nil {
			return nil, err
		}
		machines[role] = m
	}
	return machines, nil
}

// machinesFolder finds machines/ in the factory's folder: next to the
// factory's binary, or in the folder above it (the launcher builds the
// binary into .build/). It is found from the binary, not from where the
// factory is run.
func machinesFolder() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	candidates := []string{filepath.Join(dir, "machines"), filepath.Join(filepath.Dir(dir), "machines")}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("there is no machines folder: looked in %s", strings.Join(candidates, " and "))
}

func loadMachine(role, dir string) (machine, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "machine.json"))
	if err != nil {
		return machine{}, fmt.Errorf("could not read the %s's configuration: %v", role, err)
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return machine{}, fmt.Errorf("could not read the %s's machine.json: %v", role, err)
	}
	if c.Harness == "" {
		c.Harness = defaultHarness
	}
	if provider, _, _ := strings.Cut(c.Model, "/"); provider == "" || provider == c.Model {
		return machine{}, fmt.Errorf(`the %s's machine.json names no model as "<provider>/<model>" (%s)`, role, filepath.Join(dir, "machine.json"))
	}
	return machine{role, dir, c.Harness, c.Model}, nil
}

func (m machine) provider() string {
	provider, _, _ := strings.Cut(m.model, "/")
	return provider
}

// apiKeyVariables are the environment variables that hold a provider's API
// key, for the providers with a single one. A machine with no login of its
// own signs in with it.
var apiKeyVariables = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"google":     "GEMINI_API_KEY",
	"mistral":    "MISTRAL_API_KEY",
	"groq":       "GROQ_API_KEY",
	"xai":        "XAI_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
	"deepseek":   "DEEPSEEK_API_KEY",
	"cerebras":   "CEREBRAS_API_KEY",
}

// checkSignIn says so if the machine can sign in to its provider neither
// with the login in its folder nor with an API key in the environment.
func (m machine) checkSignIn() error {
	login := filepath.Join(m.dir, "auth.json")
	if raw, err := os.ReadFile(login); err == nil {
		var logins map[string]json.RawMessage
		if json.Unmarshal(raw, &logins) == nil {
			if _, ok := logins[m.provider()]; ok {
				return nil
			}
		}
	}
	key := apiKeyVariables[m.provider()]
	if key != "" && os.Getenv(key) != "" {
		return nil
	}
	hint := "copy your login into " + m.dir + "/auth.json"
	if key != "" {
		hint += ", or set " + key
	}
	return fmt.Errorf("the %s cannot sign in to %s: it has no login of its own for it, and no API key (%s)", m.role, m.provider(), hint)
}

// command is how the machine's harness is called, in dir. A harness is
// called the way pi is: pointed at the machine's folder, given the machine's
// AGENTS.md as instructions, and stopped from reading the instruction files
// around dir and the skills of whoever is running the factory.
func (m machine) command(dir, prompt string) *exec.Cmd {
	cmd := exec.Command(m.harness, "-p",
		"--no-context-files", "--no-skills",
		"--model", m.model,
		"--append-system-prompt", filepath.Join(m.dir, "AGENTS.md"),
		prompt)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+m.dir)
	return cmd
}

// run calls the machine's harness in dir and returns what it answered.
func (m machine) run(dir, prompt string) (string, error) {
	cmd := m.command(dir, prompt)
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("could not run the %s (%s): %v", m.role, m.harness, err)
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("the %s (%s) failed (%v):\n%s", m.role, m.harness, err, out.String())
	}
	return out.String(), nil
}
