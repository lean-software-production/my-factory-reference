package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The double's side of the Agent Client Protocol: JSON-RPC, a message a
// line, on its standard input and output.

// message is a request or notification from the factory.
type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

var (
	// closed is closed when the factory closes the double's input: it is
	// done with it, or has gone.
	closed = make(chan struct{})
	// The call this turn is, and what it has spent: a token for each byte it
	// was given, and for each byte it has generated.
	callNumber        int
	spentIn, spentOut int
	sessionID         = "double-1"
)

// serve answers the factory's requests until its input ends, playing a turn
// for each prompt.
func serve(play func(prompt string) error) {
	msgs := make(chan message, 100)
	go func() {
		defer close(msgs)
		defer close(closed)
		in := bufio.NewScanner(os.Stdin)
		in.Buffer(make([]byte, 1<<20), 1<<26)
		for in.Scan() {
			var m message
			if json.Unmarshal(in.Bytes(), &m) == nil {
				msgs <- m
			}
		}
	}()
	for m := range msgs {
		switch m.Method {
		case "initialize":
			reply(m.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{},
				"agentInfo": map[string]string{"name": "agent double"}})
		case "session/new":
			reply(m.ID, map[string]any{"sessionId": sessionID})
		case "session/prompt":
			var params struct {
				Prompt []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"prompt"`
			}
			if err := json.Unmarshal(m.Params, &params); err != nil {
				replyError(m.ID, err.Error())
				continue
			}
			var text []string
			for _, block := range params.Prompt {
				if block.Type == "text" {
					text = append(text, block.Text)
				}
			}
			spentIn, spentOut = 0, 0
			if err := play(strings.Join(text, "\n")); err != nil {
				replyError(m.ID, err.Error())
				continue
			}
			logUsage()
			reply(m.ID, map[string]any{"stopReason": "end_turn",
				"usage": map[string]int{"inputTokens": spentIn, "outputTokens": spentOut}})
		default:
			if len(m.ID) > 0 {
				replyError(m.ID, "Method not found: "+m.Method)
			}
		}
	}
}

// say generates a line, and keeps it in said/<call>.txt.
func say(line string) {
	spentOut += len(line) + 1
	send(map[string]any{"method": "session/update", "params": map[string]any{"sessionId": sessionID,
		"update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": line + "\n"}}}})
	if dir := os.Getenv("DOUBLE_DIR"); dir != "" && callNumber > 0 {
		appendLine(filepath.Join(dir, "said", fmt.Sprintf("%d.txt", callNumber)), line)
	}
}

// logUsage notes what the call reported it spent.
func logUsage() {
	dir := os.Getenv("DOUBLE_DIR")
	if dir == "" || callNumber == 0 {
		return
	}
	machine := filepath.Base(os.Getenv("PI_CODING_AGENT_DIR"))
	withLock(dir, func() {
		appendLine(filepath.Join(dir, "usage.log"), fmt.Sprintf("%d %s %d %d", callNumber, machine, spentIn, spentOut))
	})
}

func reply(id json.RawMessage, result any) { send(map[string]any{"id": id, "result": result}) }

func replyError(id json.RawMessage, message string) {
	send(map[string]any{"id": id, "error": map[string]any{"code": -32603, "message": message}})
}

func send(msg map[string]any) {
	msg["jsonrpc"] = "2.0"
	line, err := json.Marshal(msg)
	if err != nil {
		fail("%v", err)
	}
	os.Stdout.Write(append(line, '\n'))
}
