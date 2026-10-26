package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// The daemon runs one run at a time. Starting a run returns as soon as it
// has been checked and started; it then goes along its line in the daemon,
// keeping its record as it goes, until it reaches the end, fails or is
// stopped. Stopping a run leaves the daemon running; stopping the daemon
// stops its run too.
type daemon struct {
	mu      sync.Mutex
	current *ride // the run it is running, if any
	quit    chan struct{}
	quitter sync.Once
}

// A ride is a run going along its line in the daemon.
type ride struct {
	run    *run
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{} // closed once it has ended and its folder is committed
}

func (rd *ride) name() string   { return rd.run.folder.name }
func (rd *ride) target() string { return rd.run.folder.target }

// errStopped is why a run that was stopped stopped.
var errStopped = errors.New("the run was stopped")

// serveDaemon runs the daemon until it is stopped, by a request or by a
// signal it was not started ignoring. It tells whoever started it, on the
// readiness pipe it was given, when it is ready, or that another daemon is
// already running for this factory.
func serveDaemon(ready *os.File) error {
	tell := func(word string) {
		if ready != nil {
			fmt.Fprintln(ready, word)
			ready.Close()
		}
	}
	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// The lock is held for as long as the daemon runs, and let go however it
	// ends, so a socket left by one that crashed is known to be stale.
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		tell("running")
		return nil
	}
	os.Remove(socketPath(dir))
	listener, err := net.Listen("unix", socketPath(dir))
	if err != nil {
		return err
	}
	tell("ready")

	signalled, stop := stoppedBySignals()
	defer stop()
	d := &daemon{quit: make(chan struct{})}
	go func() {
		select {
		case <-signalled.Done():
		case <-d.quit:
		}
		listener.Close()
	}()
	var serving sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		serving.Go(func() { d.serve(conn) })
	}
	d.stopRide()
	serving.Wait()
	os.RemoveAll(dir)
	return nil
}

// serve answers one request.
func (d *daemon) serve(conn net.Conn) {
	defer conn.Close()
	var req request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	var rep reply
	switch req.Op {
	case "start":
		rep = d.start(req)
	case "stop":
		rep = d.stopRun(req)
	case "say":
		rep = d.say(req)
	case "shutdown":
		d.quitter.Do(func() { close(d.quit) })
		d.stopRide()
		rep = reply{OK: true, Message: "stopped the factory", PID: os.Getpid()}
	case "status":
		rep = d.status()
	default:
		rep = failure(fmt.Errorf("the factory does not know how to %q", req.Op))
	}
	json.NewEncoder(conn).Encode(rep)
}

func failure(err error) reply { return reply{Error: err.Error()} }

// start checks and starts the run, unless one is running already.
func (d *daemon) start(req request) reply {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current != nil {
		return failure(fmt.Errorf("refused: run %s is running", d.current.name()))
	}
	select {
	case <-d.quit:
		return failure(errors.New("refused: the factory is stopping"))
	default:
	}
	i := invocation{run: req.Run, target: req.Target, line: req.Line, seed: req.Seed, attempts: req.Attempts}
	r, err := i.prepare(req.Env)
	if err != nil {
		return failure(err)
	}
	if r.record, err = openRecord(r.folder); err != nil {
		return failure(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	rd := &ride{run: r, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	r.record.add(watch.Event{Kind: watch.Run, Run: rd.name(), State: watch.Started})
	d.current = rd
	go d.ride(rd)
	return reply{OK: true, Message: "started run " + rd.name()}
}

// ride runs the run to its end, records how it ended and commits its
// folder, however it ended.
func (d *daemon) ride(rd *ride) {
	err := rd.run.along(rd.ctx)
	end := watch.Event{Kind: watch.Run, Run: rd.name(), State: watch.Finished}
	switch {
	case errors.Is(context.Cause(rd.ctx), errStopped):
		end.State = watch.Stopped
	case err != nil:
		end.State, end.Error = watch.Failed, err.Error()
	}
	rec := rd.run.record
	rec.add(end)
	if err := commitRunFolder(rd.run.folder, "Keep what the "+rd.name()+" run made"); err != nil {
		rec.add(watch.Event{Kind: watch.Run, Run: rd.name(), State: watch.Failed, Error: err.Error()})
		fmt.Fprintf(os.Stderr, "factory: the %s run: %v\n", rd.name(), err)
	}
	rec.close()
	d.mu.Lock()
	d.current = nil
	d.mu.Unlock()
	close(rd.done)
}

// stopRun stops the run the request names, if it is the one running, and
// returns once it has stopped and its folder is committed.
func (d *daemon) stopRun(req request) reply {
	d.mu.Lock()
	rd := d.current
	d.mu.Unlock()
	if rd == nil || rd.name() != req.Run || rd.target() != req.Target {
		return failure(fmt.Errorf("the %q run is not running", req.Run))
	}
	rd.cancel(errStopped)
	<-rd.done
	return reply{OK: true, Message: "stopped run " + rd.name()}
}

// sayLimit is how long a machine's agent has to say whether it took a
// message.
const sayLimit = 10 * time.Second

// say passes the message to the machine the request names, if it is working
// now in the run that is running, and returns once its agent has taken it.
// Otherwise the message is refused: nothing is held for later.
func (d *daemon) say(req request) reply {
	d.mu.Lock()
	rd := d.current
	d.mu.Unlock()
	if rd == nil {
		return failure(errors.New("refused: nothing is running"))
	}
	if rd.name() != req.Run || rd.target() != req.Target {
		return failure(fmt.Errorf("refused: the %q run is not running (run %s is, in %s)", req.Run, rd.name(), rd.target()))
	}
	ctx, cancel := context.WithTimeout(rd.ctx, sayLimit)
	defer cancel()
	if err := rd.run.tell(ctx, req.Machine, req.Message); err != nil {
		return failure(err)
	}
	return reply{OK: true, Message: "delivered to " + req.Machine}
}

// stopRide stops the run that is running, if any, and waits until it has.
func (d *daemon) stopRide() {
	d.mu.Lock()
	rd := d.current
	d.mu.Unlock()
	if rd != nil {
		rd.cancel(errStopped)
		<-rd.done
	}
}

func (d *daemon) status() reply {
	d.mu.Lock()
	defer d.mu.Unlock()
	rep := reply{OK: true, PID: os.Getpid()}
	if d.current != nil {
		rep.Run, rep.Target = d.current.name(), d.current.target()
	}
	return rep
}
