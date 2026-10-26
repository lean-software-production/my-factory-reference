package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	// playing is whether a turn is being played. The turn and what the
	// factory tells it take turns at saying lines, holding output.
	playing bool
	output  sync.Mutex
	unkept  []string // lines said before the call had its number
)

// serve answers the factory's requests until its input ends, playing a turn
// for each prompt. While a turn is being played it goes on answering: a
// message passed to it with _session/steering it hears there and then.
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
	var turns sync.WaitGroup
	for m := range msgs {
		switch m.Method {
		case "initialize":
			reply(m.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{},
				"agentInfo": map[string]string{"name": "agent double"}})
		case "session/new":
			reply(m.ID, map[string]any{"sessionId": sessionID})
		case "session/prompt":
			prompt, err := promptText(m.Params)
			if err != nil {
				replyError(m.ID, err.Error())
				continue
			}
			output.Lock()
			spentIn, spentOut, playing = 0, 0, true
			output.Unlock()
			turns.Go(func() {
				err := play(prompt)
				output.Lock()
				playing = false
				output.Unlock()
				if err != nil {
					replyError(m.ID, err.Error())
					return
				}
				logUsage()
				reply(m.ID, map[string]any{"stopReason": "end_turn",
					"usage": map[string]int{"inputTokens": spentIn, "outputTokens": spentOut}})
			})
		case "_session/steering":
			message, err := promptText(m.Params)
			if err != nil {
				replyError(m.ID, err.Error())
				continue
			}
			if !hear(message) {
				reply(m.ID, map[string]any{"outcome": "promptRequired", "reason": "noRunningTurn"})
				continue
			}
			reply(m.ID, map[string]any{"outcome": "injected"})
		default:
			if len(m.ID) > 0 {
				replyError(m.ID, "Method not found: "+m.Method)
			}
		}
	}
	turns.Wait()
}

// promptText is the text of the prompt a request carries.
func promptText(params json.RawMessage) (string, error) {
	var p struct {
		Prompt []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", err
	}
	var text []string
	for _, block := range p.Prompt {
		if block.Type == "text" {
			text = append(text, block.Text)
		}
	}
	return strings.Join(text, "\n"), nil
}

// hear takes a message given while a turn is being played, saying
// "heard: <message>" there and then; it says whether a turn was.
func hear(message string) bool {
	output.Lock()
	defer output.Unlock()
	if !playing {
		return false
	}
	spentIn += len(message)
	sayLocked("heard: " + message)
	return true
}

// say generates a line, and keeps it in said/<call>.txt.
func say(line string) {
	output.Lock()
	defer output.Unlock()
	sayLocked(line)
}

func sayLocked(line string) {
	spentOut += len(line) + 1
	send(map[string]any{"method": "session/update", "params": map[string]any{"sessionId": sessionID,
		"update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": line + "\n"}}}})
	if callNumber == 0 {
		// Heard before the call is numbered: kept once it is.
		unkept = append(unkept, line)
		return
	}
	if dir := os.Getenv("DOUBLE_DIR"); dir != "" {
		appendLine(filepath.Join(dir, "said", fmt.Sprintf("%d.txt", callNumber)), line)
	}
}

// numbered is the call being given its number: what it said before then is
// kept now.
func numbered(n int) {
	output.Lock()
	defer output.Unlock()
	callNumber = n
	if dir := os.Getenv("DOUBLE_DIR"); dir != "" {
		for _, line := range unkept {
			appendLine(filepath.Join(dir, "said", fmt.Sprintf("%d.txt", n)), line)
		}
	}
	unkept = nil
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

// send writes a message, whole, whichever turn is sending it.
func send(msg map[string]any) {
	msg["jsonrpc"] = "2.0"
	line, err := json.Marshal(msg)
	if err != nil {
		fail("%v", err)
	}
	sending.Lock()
	defer sending.Unlock()
	os.Stdout.Write(append(line, '\n'))
}

var sending sync.Mutex
