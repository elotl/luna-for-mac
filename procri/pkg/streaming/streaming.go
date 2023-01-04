package streaming

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	k8sstreaming "k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

func NewStreamingServer(addr string) (k8sstreaming.Server, error) {
	config := k8sstreaming.DefaultConfig
	config.Addr = addr
	runtime := newStreamingRuntime()
	return k8sstreaming.NewServer(config, runtime)
}

type streamingRuntime struct {
}

func newStreamingRuntime() k8sstreaming.Runtime {
	return &streamingRuntime{}
}

type WinSize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

func setSize(fd uintptr, size remotecommand.TerminalSize) error {
	winsize := &unix.Winsize{Row: size.Height, Col: size.Width}
	return unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, winsize)
}

func ttyCmd(execCmd *exec.Cmd, stdin io.Reader, stdout io.WriteCloser, resize <-chan remotecommand.TerminalSize) error {
	// copied from cri-o
	p, err := pty.Start(execCmd)
	if err != nil {
		return err
	}
	defer p.Close()
	defer stdout.Close()
	kubecontainer.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		if err := setSize(p.Fd(), size); err != nil {
			klog.Warningf("unable to set terminal size: %v", err)
		}
	})
	var stdinErr, stdoutErr error
	if stdin != nil {
		go func() { _, stdinErr = pools.Copy(p, stdin) }()
	}
	if stdout != nil {
		go func() { _, stdoutErr = pools.Copy(stdout, p) }()
	}
	err = execCmd.Wait()
	if stdinErr != nil {
		klog.Warningf("stdin copy error: %v", stdinErr)
	}
	if stdoutErr != nil {
		klog.Warningf("stdout copy error: %v", stdoutErr)
	}
	return err
}

func (s *streamingRuntime) Exec(containerID string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	// TODO: start cmd via exec.Command() and Start(), and start forwarding stdin/stdout via a goroutine.
	if len(cmd) < 1 {
		return fmt.Errorf("empty command")
	}
	var command *exec.Cmd
	if len(cmd) == 1 {
		command = exec.Command(cmd[0])
	} else {
		command = exec.Command(cmd[0], cmd[1:]...)
	}
	var cmdErr error
	if tty {
		cmdErr = ttyCmd(command, stdin, stdout, resize)
	} else {
		command.Stdout = stdout
		command.Stderr = stderr
		command.Stdin = stdin
		cmdErr = command.Start()
		if cmdErr != nil {
			return cmdErr
		}
		cmdErr = command.Wait()
	}

	return cmdErr
}

func (s *streamingRuntime) Attach(containerID string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	// TODO: this will be hard to implement, since we don't keep subprocess
	// files open. Probably we can attach to the process via ptrace.
	return fmt.Errorf("not implemented")
}

func (s *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	defer stream.Close()
	ctx := context.TODO()
	err := handlePortForward(ctx, port, stream)
	if err != nil {
		klog.Errorf("port forwarding error: %v", err)
	}

	klog.V(3).Infof("Finished port forwarding for %q on port %d", podSandboxID, port)
	return err
}

func handlePortForward(ctx context.Context, port int32, stream io.ReadWriteCloser) error {
	// shameless copy from
	// https://github.com/cri-o/cri-o/blob/84464383a1c12a5cde27a4afc5a2b1f4eaa6b3ce/internal/oci/runtime_oci.go#L322
	conn, err := net.Dial("tcp4", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return errors.Wrapf(err, "dialing %d", port)
	}
	defer conn.Close()

	errCh := make(chan error, 2)

	// Copy from the the namespace port connection to the client stream
	go func() {
		klog.V(5).Infof("copy data from container port %d to client", port)
		_, err := io.Copy(stream, conn)
		errCh <- err
	}()

	// Copy from the client stream to the namespace port connection
	go func() {
		klog.V(5).Infof("copy data from client to container port %d", port)
		_, err := io.Copy(conn, stream)
		errCh <- err
	}()

	// Wait until the first error is returned by one of the connections we
	// use errFwd to store the result of the port forwarding operation if
	// the context is cancelled close everything and return
	var errFwd error
	select {
	case errFwd = <-errCh:
		klog.Errorf("stop forwarding in direction: %v", errFwd)
	case <-ctx.Done():
		klog.Warningf("cancelled: %v", ctx.Err())
		return ctx.Err()
	}

	// give a chance to terminate gracefully or timeout
	const timeout = time.Second
	select {
	case e := <-errCh:
		if errFwd == nil {
			errFwd = e
		}
		klog.V(5).Info("stopped forwarding in both directions")

	case <-time.After(timeout):
		klog.V(5).Info("timed out waiting to close the connection")

	case <-ctx.Done():
		klog.V(5).Infof("cancelled: %v", ctx.Err())
		errFwd = ctx.Err()
	}

	return errFwd
}
