package machine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Usage is what a turn spent, as the machine's ACP agent reports it on its
// session/prompt response.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// Turn is what a machine generated in its turn, and what that spent.
type Turn struct {
	Text  string
	Usage Usage
}

// Run starts the machine's ACP agent in dir, in the environment env, gives
// it the prompt in a session of its own, and returns what it generated and
// spent. Each line it generates is passed to said once the line is
// complete, and the last one when the turn ends, finished or not.
//
// The agent runs in a process group of its own, with whatever it starts.
// When ctx is done, because another branch failed or the run was stopped,
// the agent is stopped there and then; and once it has answered, it is told
// it is done by the end of its input, and anything it left running in its
// group, such as a server it started, is stopped too. No machine outlives
// its call. An agent that gives up before it answers, as one that cannot
// sign in does, fails the call at once, with what it said on its standard
// error.
//
// While the turn runs, the machine can be told something through steering,
// if there is one.
func (m Machine) Run(ctx context.Context, dir, prompt string, env []string, said func(line string), steering *Steering) (Turn, error) {
	complaints, err := os.CreateTemp("", "factory-harness-")
	if err != nil {
		return Turn{}, err
	}
	defer os.Remove(complaints.Name())
	defer complaints.Close()

	cmd := m.command(dir, env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Turn{}, err
	}
	// Its output is a pipe of the factory's own, which the factory closes
	// when it is done with it, so that nothing the agent leaves running can
	// keep the call open.
	out, w, err := os.Pipe()
	if err != nil {
		return Turn{}, err
	}
	defer out.Close()
	cmd.Stdout, cmd.Stderr = w, complaints
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	w.Close()
	if err != nil {
		return Turn{}, fmt.Errorf("could not run the %s (%s): %v", m.Name, m.Harness, err)
	}
	p := &process{cmd: cmd, exited: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.exited)
	}()
	done := make(chan struct{})
	defer close(done)
	s := &session{in: stdin, msgs: messages(out, done), transcript: transcript{said: said},
		waiting: map[string]chan message{}, over: make(chan struct{})}

	usage, err := s.converse(ctx, p, dir, prompt, steering)
	steering.end()
	close(s.over)
	s.transcript.flush()
	switch {
	case ctx.Err() != nil:
		p.stop()
		return Turn{}, fmt.Errorf("the %s (%s) was stopped: %v", m.Name, m.Harness, context.Cause(ctx))
	case errors.As(err, new(unanswered)):
		p.stop()
		return Turn{}, fmt.Errorf("the %s (%s) failed: %v", m.Name, m.Harness, err)
	case errors.Is(err, errGone):
		p.stop()
		return Turn{}, fmt.Errorf("the %s (%s) stopped before it answered (%v)%s", m.Name, m.Harness, exitStatus(p.err), lastWords(complaints))
	case err != nil:
		p.end(stdin)
		return Turn{}, fmt.Errorf("the %s (%s) failed: %v", m.Name, m.Harness, err)
	}
	p.end(stdin)
	text := s.transcript.text.String()
	if strings.TrimSpace(text) == "" {
		// As pi-rpc-acp ends a turn pi could not take, signed out, say.
		return Turn{}, fmt.Errorf("the %s (%s) ended its turn having generated nothing%s", m.Name, m.Harness, lastWords(complaints))
	}
	return Turn{Text: text, Usage: usage}, nil
}

// errGone is an agent's output ending, or its input closing, before it has
// answered.
var errGone = errors.New("the agent has gone")

// A session is the factory's side of an ACP conversation with one agent:
// the requests it sends, and the messages the agent sends back. The turn
// reads them; a message the machine is told while its turn runs is sent
// from elsewhere, and waits for its answer until the turn is over.
type session struct {
	in         io.Writer
	msgs       <-chan message
	transcript transcript

	// mu is held to send, so that messages go whole, and for the requests'
	// ids and who awaits their answers.
	mu      sync.Mutex
	last    int                     // the id of the latest request
	waiting map[string]chan message // the requests sent from elsewhere awaiting their answers, by id
	over    chan struct{}           // closed when the turn is over and no more answers will be read
}

// message is a JSON-RPC message from the agent: a response to one of the
// factory's requests, a notification, or a request of its own.
type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// converse has the agent take the prompt: it is initialized, a session is
// started in dir, and the prompt is sent in it. It returns what the turn
// spent.
func (s *session) converse(ctx context.Context, p *process, dir, prompt string, steering *Steering) (Usage, error) {
	if _, err := s.start(ctx, p, "initialize", map[string]any{
		"protocolVersion": 1, "clientCapabilities": map[string]any{},
	}); err != nil {
		return Usage{}, err
	}
	raw, err := s.start(ctx, p, "session/new", map[string]any{"cwd": dir, "mcpServers": []any{}})
	if err != nil {
		return Usage{}, err
	}
	var started struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &started) != nil || started.SessionID == "" {
		return Usage{}, fmt.Errorf("it started no session: %s", raw)
	}
	id, err := s.request("session/prompt", map[string]any{
		"sessionId": started.SessionID,
		"prompt":    textPrompt(prompt),
	})
	if err != nil {
		p.waitFor(stopGrace)
		return Usage{}, errGone
	}
	steering.begin(s, started.SessionID)
	if raw, err = s.await(ctx, p, id); err != nil {
		return Usage{}, err
	}
	var ended struct {
		Usage Usage `json:"usage"`
	}
	json.Unmarshal(raw, &ended) // an agent that reports no usage spent nothing the factory can count
	return ended.Usage, nil
}

// drainTime is how long the factory goes on reading an agent that has
// exited, for what it wrote before it went.
const drainTime = time.Second

func textPrompt(text string) []any {
	return []any{map[string]string{"type": "text", "text": text}}
}

// call sends the request, and reads the agent's messages until its
// response, which it returns.
func (s *session) call(ctx context.Context, p *process, method string, params any) (json.RawMessage, error) {
	id, err := s.request(method, params)
	if err != nil {
		p.waitFor(stopGrace)
		return nil, errGone
	}
	return s.await(ctx, p, id)
}

// start is a call of one of the requests that start the agent, which it
// must answer within startDeadline.
func (s *session) start(ctx context.Context, p *process, method string, params any) (json.RawMessage, error) {
	deadline, cancel := context.WithTimeout(ctx, startDeadline)
	defer cancel()
	raw, err := s.call(deadline, p, method, params)
	if err != nil && ctx.Err() == nil && deadline.Err() != nil {
		return nil, unanswered{method, startDeadline}
	}
	return raw, err
}

// unanswered is an agent not answering a request that starts it within
// startDeadline.
type unanswered struct {
	method string
	within time.Duration
}

func (u unanswered) Error() string {
	return fmt.Sprintf("it did not answer %s within %v: is it an ACP agent?", u.method, u.within)
}

// request sends a request, and returns its id.
func (s *session) request(method string, params any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last++
	return strconv.Itoa(s.last), s.sendLocked(map[string]any{"jsonrpc": "2.0", "id": s.last, "method": method, "params": params})
}

// await reads the agent's messages until its response to the request with
// that id, which it returns. Responses to requests sent from elsewhere are
// passed to whoever is waiting for them.
func (s *session) await(ctx context.Context, p *process, id string) (json.RawMessage, error) {
	var gone <-chan time.Time // after the agent has exited
	exited := p.exited
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-exited:
			exited, gone = nil, time.After(drainTime)
		case <-gone:
			return nil, errGone
		case m, ok := <-s.msgs:
			if !ok {
				return nil, errGone
			}
			if m.Method == "" && string(m.ID) == id {
				if m.Error != nil {
					return nil, errors.New(m.Error.Message)
				}
				return m.Result, nil
			}
			if m.Method == "" && s.answered(m) {
				continue
			}
			if err := s.handle(m); err != nil {
				return nil, errGone
			}
		}
	}
}

// handle reads what the agent generates from its session/update
// notifications. A request of the agent's own, such as asking permission,
// is refused: the factory offers its machines nothing but the prompt.
func (s *session) handle(m message) error {
	if m.Method == "session/update" {
		var update struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		}
		if json.Unmarshal(m.Params, &update) == nil && update.Update.SessionUpdate == "agent_message_chunk" && update.Update.Content.Type == "text" {
			s.transcript.add(update.Update.Content.Text)
		}
		return nil
	}
	if m.Method != "" && len(m.ID) > 0 {
		return s.send(map[string]any{"jsonrpc": "2.0", "id": m.ID,
			"error": map[string]any{"code": -32601, "message": "the factory does not offer " + m.Method}})
	}
	return nil
}

// answered passes the response to the request sent from elsewhere that
// awaits it, if one does.
func (s *session) answered(m message) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	answer, ok := s.waiting[string(m.ID)]
	if ok {
		delete(s.waiting, string(m.ID))
		answer <- m
	}
	return ok
}

func (s *session) send(msg map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendLocked(msg)
}

func (s *session) sendLocked(msg map[string]any) error {
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = s.in.Write(append(line, '\n'))
	return err
}

// messages reads the agent's output, one JSON-RPC message a line, until it
// ends or the factory is done with it. A line that is not JSON is not a
// message, and is passed over.
func messages(out io.Reader, done <-chan struct{}) <-chan message {
	msgs := make(chan message)
	go func() {
		defer close(msgs)
		r := bufio.NewReader(out)
		for {
			line, err := r.ReadBytes('\n')
			var m message
			if len(line) > 0 && json.Unmarshal(line, &m) == nil {
				select {
				case msgs <- m:
				case <-done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return msgs
}

// transcript is what a machine generated, kept whole and passed on a line
// at a time as it arrives in chunks.
type transcript struct {
	said    func(line string)
	text    strings.Builder
	pending string // the line it is in the middle of
}

func (t *transcript) add(chunk string) {
	t.text.WriteString(chunk)
	t.pending += chunk
	for {
		line, rest, ok := strings.Cut(t.pending, "\n")
		if !ok {
			return
		}
		t.pending = rest
		t.say(line)
	}
}

// flush passes on the line the machine was in the middle of, if any.
func (t *transcript) flush() {
	if t.pending != "" {
		t.say(t.pending)
		t.pending = ""
	}
}

func (t *transcript) say(line string) {
	if t.said != nil {
		t.said(strings.TrimRight(line, "\r"))
	}
}

// lastWords are the last of what the agent wrote on its standard error,
// to say why it gave up.
func lastWords(complaints *os.File) string {
	raw, err := os.ReadFile(complaints.Name())
	text := strings.TrimSpace(string(raw))
	if err != nil || text == "" {
		return ""
	}
	if lines := strings.Split(text, "\n"); len(lines) > 20 {
		text = strings.Join(lines[len(lines)-20:], "\n")
	}
	return ": " + text
}

func exitStatus(err error) string {
	if err == nil {
		return "exit status 0"
	}
	return err.Error()
}

// process is an agent's process, and what it exited with once it has.
type process struct {
	cmd    *exec.Cmd
	exited chan struct{} // closed once it has exited, when err is set
	err    error
	once   sync.Once
}

// waitFor says whether the process exits within d.
func (p *process) waitFor(d time.Duration) bool {
	select {
	case <-p.exited:
		return true
	case <-time.After(d):
		return false
	}
}

// stop stops the agent: SIGTERM to its group, and stopGrace to clean up,
// because pi runs its tools' commands in sessions of their own, out of reach
// of its group, and kills them itself on SIGTERM; then SIGKILL. Whatever is
// still in its group after that is stopped too.
func (p *process) stop() {
	p.once.Do(func() {
		if !p.waitFor(0) {
			signalGroup(p.cmd.Process, syscall.SIGTERM)
			if !p.waitFor(stopGrace) {
				signalGroup(p.cmd.Process, syscall.SIGKILL)
				<-p.exited
			}
		}
		stopGroup(p.cmd.Process)
	})
}

// end ends an agent that has answered. The end of its input tells it it is
// done; one still there after stopGrace is stopped.
func (p *process) end(stdin io.Closer) {
	stdin.Close()
	p.waitFor(stopGrace)
	p.stop()
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

// startDeadline is how long an agent is given to answer each of the
// requests that start it, initialize and session/new. One that is not an
// ACP agent, as plain pi is not, never answers them.
var startDeadline = 30 * time.Second
