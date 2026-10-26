package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The factory runs as a daemon, one for each copy of the factory. The
// commands that start, watch and stop runs talk to it over a Unix socket,
// in a folder of its own in the system's temporary folder, named for where
// the factory's program is. The first command that needs the daemon starts
// it, and it keeps running, between runs too, until it is stopped.

// request is what a command asks the daemon.
type request struct {
	Op       string   `json:"op"` // start, stop, shutdown or status
	Run      string   `json:"run,omitempty"`
	Target   string   `json:"target,omitempty"` // an absolute path
	Line     string   `json:"line,omitempty"`
	Seed     string   `json:"seed,omitempty"` // an absolute path
	Attempts int      `json:"attempts,omitempty"`
	Env      []string `json:"env,omitempty"` // the caller's, for the run's machines
}

// reply is what the daemon answers.
type reply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	PID     int    `json:"pid,omitempty"`    // the daemon's
	Run     string `json:"run,omitempty"`    // the run it is running, if any
	Target  string `json:"target,omitempty"` // and its target
}

// stateDir is the daemon's folder: its socket, its lock and its log.
func stateDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	sum := sha256.Sum256([]byte(exe))
	return filepath.Join(os.TempDir(), fmt.Sprintf("factory-%d", os.Getuid()), hex.EncodeToString(sum[:6])), nil
}

func socketPath(dir string) string { return filepath.Join(dir, "socket") }

var errNotRunning = errors.New("the factory is not running")

// call sends the request to the daemon and returns its reply. A daemon
// that is not running is started if start is set; otherwise the call fails
// with errNotRunning.
func call(req request, start bool) (reply, error) {
	dir, err := stateDir()
	if err != nil {
		return reply{}, err
	}
	conn, err := net.Dial("unix", socketPath(dir))
	if err != nil {
		if !start {
			return reply{}, errNotRunning
		}
		if err := startDaemon(dir); err != nil {
			return reply{}, err
		}
		if conn, err = net.Dial("unix", socketPath(dir)); err != nil {
			return reply{}, fmt.Errorf("could not reach the factory: %v", err)
		}
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return reply{}, fmt.Errorf("could not reach the factory: %v", err)
	}
	var rep reply
	if err := json.NewDecoder(conn).Decode(&rep); err != nil {
		return reply{}, fmt.Errorf("the factory did not answer: %v", err)
	}
	if !rep.OK {
		return rep, errors.New(rep.Error)
	}
	return rep, nil
}

// daemonStartLimit is how long a daemon has to say it is ready.
const daemonStartLimit = 10 * time.Second

// startDaemon starts the daemon, in a session of its own so that it outlives
// the terminal that started it, and waits until it is ready. A signal the
// caller ignores, as nohup has it ignore SIGHUP, the daemon ignores too.
func startDaemon(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "daemon.log")
	log, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	ready, readyW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer ready.Close()
	daemon := exec.Command(exe, "daemon", "--ready-fd", "3")
	daemon.Dir = "/"
	daemon.Stdout, daemon.Stderr = log, log
	daemon.ExtraFiles = []*os.File{readyW}
	daemon.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = daemon.Start()
	readyW.Close()
	if err != nil {
		return fmt.Errorf("could not start the factory: %v", err)
	}
	go daemon.Wait() // reaped if it stops while this command still runs

	said := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(ready).ReadString('\n')
		said <- strings.TrimSpace(line)
	}()
	select {
	case line := <-said:
		if line == "ready" || line == "running" {
			return nil
		}
	case <-time.After(daemonStartLimit):
	}
	raw, _ := os.ReadFile(logPath)
	return fmt.Errorf("the factory did not start (its log is %s):\n%s", logPath, lastLines(string(raw), 20))
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// waitUntilGone waits for the process to exit, for up to the limit.
func waitUntilGone(pid int, limit time.Duration) bool {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if processGone(pid) {
			return true
		}
	}
	return false
}

// processGone says whether the process has exited: it is not there, or it
// is a zombie its parent has yet to reap, where the system says so (Linux's
// /proc).
func processGone(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	_, after, _ := strings.Cut(string(stat), ") ")
	return strings.HasPrefix(after, "Z")
}
