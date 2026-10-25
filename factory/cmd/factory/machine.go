package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// A machine does one kind of job for the factory: the planner keeps the
// plan, the doer does the work, and the validator checks it. Each is run by
// a harness, a coding agent called as: <harness> -p <prompt>.
type machine struct {
	role    string
	harness string
}

// run calls the machine's harness in dir and returns what it answered.
func (m machine) run(dir, prompt string) (string, error) {
	cmd := exec.Command(m.harness, "-p", prompt)
	cmd.Dir = dir
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
