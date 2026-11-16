package gui

import (
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Restarting after an update starts the new binary and quits this one. The
// new process would otherwise find this one still holding the single-
// instance lock and just hand over to it, so it's told this process's id
// and waits for it to exit first.
const restartWaitEnv = "LAZYTRACKIT_RESTART_WAIT"

// relaunch starts exe as a new, independent LazyTrackIt that waits for this
// process to exit. The window shows, since the user asked for the restart.
func relaunch(exe string) error {
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), restartWaitEnv+"="+strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// waitForRestartedProcess waits (briefly) for the process that relaunched
// this one to exit; it returns at once on a normal start.
func waitForRestartedProcess() {
	pid, err := strconv.Atoi(os.Getenv(restartWaitEnv))
	os.Unsetenv(restartWaitEnv)
	if err != nil || pid <= 0 {
		return
	}
	deadline := time.Now().Add(15 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}
