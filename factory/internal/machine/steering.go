package machine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
)

// Steering is how a machine is told something while it works: a message
// for the turn it is taking, passed to its ACP agent with _session/steering,
// the extension the Claude Code and Codex adapters and pi-rpc-acp accept.
// The agent takes it in at its next step. A message lasts as long as the
// turn that heard it; nothing is kept for a turn to come.
type Steering struct {
	mu        sync.Mutex
	session   *session // while the turn runs
	sessionID string
	ready     chan struct{} // closed once the turn has begun, or is over
	once      sync.Once
}

// NewSteering is the steering for one turn of a machine, to be given to
// Run.
func NewSteering() *Steering { return &Steering{ready: make(chan struct{})} }

// ErrNotWorking is a message for a machine whose turn is over, or never
// began.
var ErrNotWorking = errors.New("it is not working")

// begin is the turn beginning: its prompt has been sent in the session.
func (st *Steering) begin(s *session, sessionID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	st.session, st.sessionID = s, sessionID
	st.mu.Unlock()
	st.once.Do(func() { close(st.ready) })
}

// end is the turn ending, however it ended.
func (st *Steering) end() {
	if st == nil {
		return
	}
	st.mu.Lock()
	st.session = nil
	st.mu.Unlock()
	st.once.Do(func() { close(st.ready) })
}

// Tell passes the words to the machine's turn, and returns once its agent
// has said they went in. It fails if the turn is over, or the agent
// would not take it.
func (st *Steering) Tell(ctx context.Context, words string) error {
	select {
	case <-st.ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	st.mu.Lock()
	s, sessionID := st.session, st.sessionID
	st.mu.Unlock()
	if s == nil {
		return ErrNotWorking
	}
	id, answer, err := s.requestAside("_session/steering", map[string]any{
		"sessionId": sessionID,
		"prompt":    textPrompt(words),
	})
	if err != nil {
		return ErrNotWorking
	}
	var m message
	select {
	case m = <-answer:
	case <-ctx.Done():
		s.forget(id)
		return ctx.Err()
	case <-s.over:
		select {
		case m = <-answer:
		default:
			return fmt.Errorf("%w: its turn ended before it said whether it heard", ErrNotWorking)
		}
	}
	if m.Error != nil {
		return fmt.Errorf("its agent refused it: %s", m.Error.Message)
	}
	var steered struct {
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	json.Unmarshal(m.Result, &steered)
	switch steered.Outcome {
	case "injected":
		return nil
	case "":
		return fmt.Errorf("its agent did not say whether it took it: %s", m.Result)
	case "promptRequired":
		// As an agent says of a turn that has just ended.
		return fmt.Errorf("%w: its agent says it has no turn running (%s)", ErrNotWorking, steered.Reason)
	}
	return fmt.Errorf("its agent did not take it (%s %s)", steered.Outcome, steered.Reason)
}

// requestAside sends a request while the turn reads the agent's messages,
// and returns its id and where its response will be passed.
func (s *session) requestAside(method string, params any) (string, <-chan message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.over:
		return "", nil, ErrNotWorking
	default:
	}
	s.last++
	id := strconv.Itoa(s.last)
	answer := make(chan message, 1)
	s.waiting[id] = answer
	if err := s.sendLocked(map[string]any{"jsonrpc": "2.0", "id": s.last, "method": method, "params": params}); err != nil {
		delete(s.waiting, id)
		return "", nil, err
	}
	return id, answer, nil
}

// forget stops waiting for the response to the request.
func (s *session) forget(id string) {
	s.mu.Lock()
	delete(s.waiting, id)
	s.mu.Unlock()
}
