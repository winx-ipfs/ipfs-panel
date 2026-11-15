package node

import (
	"io"
	"os/exec"
	"time"
)

// running is one Kubo daemon the manager spawned or adopted.
type running struct {
	pid     int
	cmd     *exec.Cmd
	stderr  *tailBuf
	logs    io.Writer
	done    chan struct{}
	waitErr error
}

func (r *running) exited() bool {
	if r == nil {
		return true
	}
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func (r *running) flushLogs() {
	if r == nil || r.logs == nil {
		return
	}
	if f, ok := r.logs.(interface{ Flush() }); ok {
		f.Flush()
	}
}

func (r *running) stderrString() string {
	if r == nil || r.stderr == nil {
		return ""
	}
	return r.stderr.String()
}

func spawnDaemon(bin, repo string, kubo io.Writer) (*running, error) {
	// Argument vector only: the binary, then "daemon". No shell, no extra flags.
	cmd := exec.Command(bin, "daemon")
	cmd.Env = withIPFSPath(nilEnv(), repo)
	stderr := newTail(detailLimit)
	stdout := io.Writer(io.Discard)
	errOut := io.Writer(stderr)
	if kubo != nil {
		stdout = kubo
		errOut = io.MultiWriter(stderr, kubo)
	}
	cmd.Stdout = stdout
	cmd.Stderr = errOut
	cmd.SysProcAttr = daemonSysProcAttr()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	child := &running{
		pid:    cmd.Process.Pid,
		cmd:    cmd,
		stderr: stderr,
		logs:   kubo,
		done:   make(chan struct{}),
	}
	go func() {
		child.waitErr = cmd.Wait()
		close(child.done)
	}()
	return child, nil
}

func nilEnv() []string {
	return osEnviron()
}

func adoptProcess(pid int) *running {
	child := &running{
		pid:    pid,
		stderr: newTail(detailLimit),
		done:   make(chan struct{}),
	}
	go func() {
		for pidAlive(pid) {
			time.Sleep(50 * time.Millisecond)
		}
		close(child.done)
	}()
	return child
}
