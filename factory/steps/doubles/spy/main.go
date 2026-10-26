// Command spy stands in for a machine's ACP agent, and runs the real one,
// pi-rpc-acp, with the same arguments, passing the Agent Client Protocol
// through both ways. It is started the way the factory starts an ACP agent,
// in the target folder. When the factory sends it a prompt, it notes the
// work in the target and the plan the prompt names; when the agent answers,
// it notes them again, so a step can tell whether the machine changed
// either. Anything else the machine keeps in the run's folder, in the
// target's .assembly-lines, is neither.
//
// It appends one line to spy.log in the folder named by DOUBLE_DIR for each
// prompt: the target's state before, then after.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	agent := exec.Command("pi-rpc-acp", os.Args[1:]...)
	agent.Stderr = os.Stderr
	toAgent, err := agent.StdinPipe()
	if err != nil {
		fail(err)
	}
	fromAgent, err := agent.StdoutPipe()
	if err != nil {
		fail(err)
	}
	if err := agent.Start(); err != nil {
		fail(err)
	}

	var (
		mu     sync.Mutex
		prompt string // the id of the prompt the agent is answering, if any
		plan   string
		before string
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		out := bufio.NewReader(fromAgent)
		for {
			line, err := out.ReadBytes('\n')
			if len(line) > 0 {
				var m struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
				}
				json.Unmarshal(line, &m)
				mu.Lock()
				if m.Method == "" && prompt != "" && string(m.ID) == prompt {
					note(before, state(plan))
					prompt = ""
				}
				mu.Unlock()
				os.Stdout.Write(line)
			}
			if err != nil {
				return
			}
		}
	}()

	in := bufio.NewReader(os.Stdin)
	for {
		line, err := in.ReadBytes('\n')
		if len(line) > 0 {
			var m struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
				Params struct {
					Prompt []struct {
						Text string `json:"text"`
					} `json:"prompt"`
				} `json:"params"`
			}
			json.Unmarshal(line, &m)
			if m.Method == "session/prompt" {
				var text []string
				for _, block := range m.Params.Prompt {
					text = append(text, block.Text)
				}
				mu.Lock()
				prompt, plan = string(m.ID), planIn(strings.Join(text, "\n"))
				before = state(plan)
				mu.Unlock()
			}
			toAgent.Write(line)
		}
		if err != nil {
			break
		}
	}
	toAgent.Close()
	<-done
	if err := agent.Wait(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fail(err)
	}
}

func note(before, after string) {
	log, err := os.OpenFile(os.Getenv("DOUBLE_DIR")+"/spy.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(log, "%s %s\n", before, after)
	log.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "spy:", err)
	os.Exit(1)
}

// planIn is the plan's path, as the factory's prompt says it.
func planIn(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		for _, prefix := range []string{"The plan is ", "There is no plan yet. It goes in "} {
			if path, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(path)
			}
		}
	}
	return ""
}

// state hashes the plan, and everything about the target's work that Git can
// see: which files changed, how, and what the untracked ones hold.
func state(plan string) string {
	sum := sha256.New()
	if raw, err := os.ReadFile(plan); err == nil {
		sum.Write(raw)
	}
	const work = ":(exclude).assembly-lines"
	for _, args := range [][]string{
		{"status", "--short", "-uall", "--", ".", work},
		{"diff", "HEAD", "--", ".", work},
	} {
		out, _ := exec.Command("git", args...).CombinedOutput()
		sum.Write(out)
	}
	untracked, _ := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z", "--", ".", work).Output()
	for _, name := range splitNul(untracked) {
		raw, _ := os.ReadFile(name)
		fmt.Fprintf(sum, "%s\n", name)
		sum.Write(raw)
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}

func splitNul(raw []byte) []string {
	var names []string
	start := 0
	for i, b := range raw {
		if b == 0 {
			names = append(names, string(raw[start:i]))
			start = i + 1
		}
	}
	return names
}
