package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// A machine does one kind of job for the factory: the planner keeps the
// plan, the doer does the work, and the validator checks it. Each is named
// on the assembly line and has a folder of its own beside the line's graph,
// machines/<name>/, which is the configuration of the harness that runs it:
// its AGENTS.md, its machine.json, and its own copy of your login.
type machine struct {
	name    string
	dir     string // the machine's folder
	harness string // the command that runs it
	model   string // <provider>/<model>
}

// config is machine.json.
type config struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// defaultHarness runs a machine whose machine.json names none.
const defaultHarness = "pi"

// loadMachines reads the folders of the machines on the line.
func loadMachines(l *assemblyLine) (map[string]machine, error) {
	machines := map[string]machine{}
	for _, name := range l.machinesOn() {
		m, err := loadMachine(name, l.machineDir(name))
		if err != nil {
			return nil, err
		}
		machines[name] = m
	}
	return machines, nil
}

func loadMachine(name, dir string) (machine, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "machine.json"))
	if err != nil {
		return machine{}, fmt.Errorf("could not read the %s's configuration: %v", name, err)
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return machine{}, fmt.Errorf("could not read the %s's machine.json: %v", name, err)
	}
	if c.Harness == "" {
		c.Harness = defaultHarness
	}
	if provider, _, _ := strings.Cut(c.Model, "/"); provider == "" || provider == c.Model {
		return machine{}, fmt.Errorf(`the %s's machine.json names no model as "<provider>/<model>" (%s)`, name, filepath.Join(dir, "machine.json"))
	}
	return machine{name, dir, c.Harness, c.Model}, nil
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
	return fmt.Errorf("the %s cannot sign in to %s: it has no login of its own for it, and no API key (%s)", m.name, m.provider(), hint)
}

// command is how the machine's harness is called, in dir. A harness is
// called the way pi is: pointed at the machine's folder, given the machine's
// AGENTS.md as instructions, and stopped from reading the instruction files
// around dir and the skills of whoever is running the factory.
func (m machine) command(ctx context.Context, dir, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, m.harness, "-p",
		"--no-context-files", "--no-skills",
		"--model", m.model,
		"--append-system-prompt", filepath.Join(m.dir, "AGENTS.md"),
		prompt)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+m.dir)
	return cmd
}

// run calls the machine's harness in dir and returns what it answered.
//
// The harness runs in a process group of its own, with whatever it starts.
// When ctx is done, because another branch failed or the factory was
// stopped, the harness is stopped; and once it has answered, anything it
// left running in its group, such as a server it started, is stopped too.
// No machine outlives its call. Its answer goes to a file, not a pipe, so
// that nothing it leaves running can hold the call open.
func (m machine) run(ctx context.Context, dir, prompt string) (string, error) {
	out, err := os.CreateTemp("", "factory-answer-")
	if err != nil {
		return "", err
	}
	defer os.Remove(out.Name())
	defer out.Close()

	cmd := m.command(ctx, dir, prompt)
	cmd.Stdout, cmd.Stderr = out, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Asked to stop, the harness gets SIGTERM, and a while to clean up: pi
	// runs its tools' commands in sessions of their own, out of reach of
	// its group, and kills them itself on SIGTERM. A harness still running
	// after that is killed.
	cmd.Cancel = func() error { return signalGroup(cmd.Process, syscall.SIGTERM) }
	cmd.WaitDelay = stopGrace
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("could not run the %s (%s): %v", m.name, m.harness, err)
	}
	err = cmd.Wait()
	stopGroup(cmd.Process)
	answer, readErr := os.ReadFile(out.Name())
	if ctx.Err() != nil {
		return "", fmt.Errorf("the %s (%s) was stopped: %v", m.name, m.harness, context.Cause(ctx))
	}
	if err != nil {
		return "", fmt.Errorf("the %s (%s) failed (%v):\n%s", m.name, m.harness, err, answer)
	}
	return string(answer), readErr
}

// stopGrace is how long what a machine runs is given to stop when asked,
// before it is killed.
const stopGrace = 3 * time.Second

// stopGroup stops whatever is still running in the process group the
// process led: SIGTERM, then, for anything still there after stopGrace,
// SIGKILL.
func stopGroup(p *os.Process) {
	if signalGroup(p, syscall.SIGTERM) != nil {
		return // nothing is left in the group
	}
	for deadline := time.Now().Add(stopGrace); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if signalGroup(p, 0) != nil {
			return
		}
	}
	signalGroup(p, syscall.SIGKILL)
}

// signalGroup sends the signal to the process group the process leads.
func signalGroup(p *os.Process, sig syscall.Signal) error {
	err := syscall.Kill(-p.Pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
