package runtimeservice

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"golang.org/x/net/context"

	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
)

const (
	defaultExecSyncTimeout int64 = 120 // seconds
)

//
// Implementation of streaming calls in cri.Runtimeservice.
//

// ExecSync runs a command in a container synchronously.
func (rs *RuntimeService) ExecSync(ctx context.Context, req *cri.ExecSyncRequest) (*cri.ExecSyncResponse, error) {
	// based on https://medium.com/@vCabbage/go-timeout-commands-with-os-exec-commandcontext-ba0c861ed738
	klog.V(4).Infof("ExecSync %v", req)
	if len(req.Cmd) < 1 {
		return nil, fmt.Errorf("exec command empty: %s", req.Cmd)
	}
	timeout := time.Duration(defaultExecSyncTimeout)
	if req.Timeout != 0 {
		timeout = time.Duration(req.Timeout)
	}

	// Create a new context and add a timeout to it
	childCtx, cancel := context.WithTimeout(ctx, timeout*time.Second)
	defer cancel() // The cancel should be deferred so resources are cleaned up

	var stdErr bytes.Buffer
	// Create the command with our context
	cmd := exec.CommandContext(childCtx, req.Cmd[0], req.Cmd[1:]...)
	cmd.Stderr = &stdErr

	// This time we can simply use Output() to get the result.
	out, err := cmd.Output()

	// We want to check the context error to see if the timeout was executed.
	// The error returned by cmd.Output() will be OS specific based on what
	// happens when a process is killed.
	if childCtx.Err() == context.DeadlineExceeded {
		klog.Errorf("ExecSync %v timeout error: %v", req, childCtx.Err())
		return &cri.ExecSyncResponse{
			Stdout:   out,
			Stderr:   stdErr.Bytes(),
			ExitCode: -1,
		}, fmt.Errorf("command timed out")
	}

	// If there's no context error, we know the command completed (or errored).
	if err != nil {
		exitCode := -1
		if exitCodeErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitCodeErr.ExitCode()
		}
		klog.Errorf("ExecSync %v error: %v", req, err)
		return &cri.ExecSyncResponse{
			Stdout:   out,
			Stderr:   stdErr.Bytes(),
			ExitCode: int32(exitCode),
		}, err
	}
	resp := &cri.ExecSyncResponse{
		Stdout:   out,
		Stderr:   stdErr.Bytes(),
		ExitCode: 0,
	}
	klog.V(4).Infof("ExecSync %v: %v", req, resp)
	return resp, nil
}

// Exec prepares a streaming endpoint to execute a command in the container.
// The actual logic is implemented in pkg/streaming/streaming.go.
func (rs *RuntimeService) Exec(ctx context.Context, req *cri.ExecRequest) (*cri.ExecResponse, error) {
	klog.V(4).Infof("Exec %v", req)

	resp, err := rs.streamingServer.GetExec(req)
	if err != nil {
		klog.Errorf("Exec %v error: %v", req, err)
	} else {
		klog.V(4).Infof("Exec %v: %v", req, resp)
	}

	return resp, err
}

// Attach prepares a streaming endpoint to attach to a running container.
// The actual logic is implemented in pkg/streaming/streaming.go.
func (rs *RuntimeService) Attach(ctx context.Context, req *cri.AttachRequest) (*cri.AttachResponse, error) {
	klog.V(4).Infof("Attach %v", req)

	resp, err := rs.streamingServer.GetAttach(req)
	if err != nil {
		klog.Errorf("Attach %v error: %v", req, err)
	} else {
		klog.V(4).Infof("Attach %v: %v", req, resp)
	}

	return resp, err
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
// The actual logic is implemented in pkg/streaming/streaming.go.
func (rs *RuntimeService) PortForward(ctx context.Context, req *cri.PortForwardRequest) (*cri.PortForwardResponse, error) {
	klog.V(4).Infof("PortForward %v", req)

	resp, err := rs.streamingServer.GetPortForward(req)
	if err != nil {
		klog.Errorf("PortForward %v error: %v", req, err)
	} else {
		klog.V(4).Infof("PortForward %v: %v", req, resp)
	}

	return resp, err
}
