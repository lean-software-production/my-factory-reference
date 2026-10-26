// Package machine runs the machines on an assembly line. Each is a folder of
// its own, the configuration of the harness that runs it, and each runs as
// an ACP agent: the factory speaks the Agent Client Protocol to it over its
// standard input and output.
package machine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// A Machine does one kind of job for the factory: the planner keeps the
// plan, the doer does the work, and the validator checks it. Each is named
// on the assembly line and has a folder of its own beside the line's graph,
// machines/<name>/, which is the configuration of the harness that runs it:
// its AGENTS.md, its machine.json, and its own copy of your login.
type Machine struct {
	Name    string
	Dir     string // the machine's folder
	Harness string // the ACP agent that runs it
	Model   string // <provider>/<model>
}

// config is machine.json.
type config struct {
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

// DefaultHarness runs a machine whose machine.json names none: pi, through
// its ACP bridge.
const DefaultHarness = "pi-rpc-acp"

// Load reads the machine.json in the machine's folder.
func Load(name, dir string) (Machine, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "machine.json"))
	if err != nil {
		return Machine{}, fmt.Errorf("could not read the %s's configuration: %v", name, err)
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Machine{}, fmt.Errorf("could not read the %s's machine.json: %v", name, err)
	}
	if c.Harness == "" {
		c.Harness = DefaultHarness
	}
	if filepath.Base(c.Harness) == "pi" {
		return Machine{}, fmt.Errorf("the %s's harness pi is not an ACP agent: use %s", name, DefaultHarness)
	}
	if provider, _, _ := strings.Cut(c.Model, "/"); provider == "" || provider == c.Model {
		return Machine{}, fmt.Errorf(`the %s's machine.json names no model as "<provider>/<model>" (%s)`, name, filepath.Join(dir, "machine.json"))
	}
	return Machine{name, dir, c.Harness, c.Model}, nil
}

// Provider is the provider of the machine's model, whose bill its tokens
// go on.
func (m Machine) Provider() string {
	provider, _, _ := strings.Cut(m.Model, "/")
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

// CheckSignIn says so if the machine can sign in to its provider neither
// with the login in its folder nor with an API key in the environment it
// is to run in.
func (m Machine) CheckSignIn(env []string) error {
	login := filepath.Join(m.Dir, "auth.json")
	if raw, err := os.ReadFile(login); err == nil {
		var logins map[string]json.RawMessage
		if json.Unmarshal(raw, &logins) == nil {
			if _, ok := logins[m.Provider()]; ok {
				return nil
			}
		}
	}
	key := apiKeyVariables[m.Provider()]
	if key != "" && getenv(env, key) != "" {
		return nil
	}
	hint := "copy your login into " + m.Dir + "/auth.json"
	if key != "" {
		hint += ", or set " + key
	}
	return fmt.Errorf("the %s cannot sign in to %s: it has no login of its own for it, and no API key (%s)", m.Name, m.Provider(), hint)
}

// bridges are the ACP agents that run another program, which must be on
// the PATH too: pi-rpc-acp runs pi.
var bridges = map[string]string{"pi-rpc-acp": "pi"}

// CheckHarness says so if the machine's ACP agent cannot be run in the
// environment it is to run in: it is not there, or, for a bridge, what it
// runs is not on the PATH. An agent that cannot start what it bridges to
// may never answer, so this is checked before the machine is called.
func (m Machine) CheckHarness(env []string) error {
	if env == nil {
		env = os.Environ()
	}
	path, err := lookPath(m.Harness, env)
	if err == nil {
		err = executable(path)
	}
	if err != nil {
		return fmt.Errorf("could not run the %s (%s): %v", m.Name, m.Harness, err)
	}
	if program, ok := bridges[filepath.Base(m.Harness)]; ok {
		if _, err := lookPath(program, env); err != nil {
			return fmt.Errorf("could not run the %s (%s): it runs %s, and %v", m.Name, m.Harness, program, err)
		}
	}
	return nil
}

func executable(path string) error {
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return fmt.Errorf("there is no %s", path)
	case info.IsDir() || info.Mode()&0o111 == 0:
		return fmt.Errorf("%s is not a program", path)
	}
	return nil
}

// command is how the machine's ACP agent is started, in dir, in the
// environment env. An agent is started the way pi-rpc-acp is: pointed at
// the machine's folder, and given arguments it passes on to pi, which give
// the machine its model and its AGENTS.md as instructions, and stop it
// reading the instruction files around dir and the skills of whoever is
// running the factory. The prompt is not an argument: it is sent over ACP.
func (m Machine) command(dir string, env []string) *exec.Cmd {
	if env == nil {
		env = os.Environ()
	}
	cmd := &exec.Cmd{
		Args: []string{m.Harness,
			"--no-context-files", "--no-skills",
			"--model", m.Model,
			"--append-system-prompt", filepath.Join(m.Dir, "AGENTS.md")},
		Dir: dir,
		Env: append(slices.Clone(env), "PI_CODING_AGENT_DIR="+m.Dir),
	}
	cmd.Path, cmd.Err = lookPath(m.Harness, env)
	return cmd
}

// lookPath finds the program on the PATH of the environment it is to run
// in, which is the caller's, not necessarily the factory's own.
func lookPath(name string, env []string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}
	for _, dir := range filepath.SplitList(getenv(env, "PATH")) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, name)
		if executable(path) == nil {
			return path, nil
		}
	}
	return name, fmt.Errorf("%s is not on the PATH", name)
}

// getenv is the value of the variable in the environment, the last one
// set winning, as it does for a process.
func getenv(env []string, name string) string {
	value := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			value = v
		}
	}
	return value
}
