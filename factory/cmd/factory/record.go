package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// recordFile is the run's record, in its folder beside its plan: what every
// machine was given, generated and spent, and when the run started and
// ended, one event a line, in the order they happened. It is committed with
// the rest of the run's folder, and watching a run reads it.
const recordFile = "record.jsonl"

func (f runFolder) record() string { return filepath.Join(f.dir, recordFile) }

// A record is a run's record, open for adding to. Machines running at once
// take turns.
type record struct {
	mu   sync.Mutex
	file *os.File
}

func openRecord(f runFolder) (*record, error) {
	file, err := os.OpenFile(f.record(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("could not open the run's record: %v", err)
	}
	return &record{file: file}, nil
}

// add writes the event to the record, as it happens. A record that cannot
// be written to is said so in the factory's log; the run goes on.
func (r *record) add(ev watch.Event) {
	if r == nil {
		return
	}
	ev.Time = time.Now().UTC()
	line, err := json.Marshal(ev)
	if err == nil {
		r.mu.Lock()
		_, err = r.file.Write(append(line, '\n'))
		r.mu.Unlock()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "factory: could not add to the run's record:", err)
	}
}

func (r *record) close() {
	if r != nil {
		r.file.Close()
	}
}
