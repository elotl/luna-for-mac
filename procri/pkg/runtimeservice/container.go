package runtimeservice

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/rs/xid"
	"golang.org/x/net/context"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
)

const (
	containerSubdir = "container/"
	containerPrefix = "cnt_"
	defaultPath     = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

var (
	containerPathAllowList    = []string{}
	containerPathDisallowList = []string{
		"/etc",
		"/usr",
		"/bin",
		"/sbin",
		"/Library",
	}
)

type Container struct {
	ID          string             `json:"id"`
	PodID       string             `json:"podID"`
	Name        string             `json:"name"`
	Attempt     uint32             `json:"attempt"`
	Args        []string           `json:"args"`
	Command     []string           `json:"command"`
	Env         []string           `json:"env"`
	WorkingDir  string             `json:"workingDir"`
	LogPath     string             `json:"logPath"`
	Pid         int                `json:"pid"`
	CreatedAt   int64              `json:"createdAt"`
	StartedAt   int64              `json:"startedAt"`
	FinishedAt  int64              `json:"finishedAt"`
	ExitCode    int32              `json:"exitCode"`
	Image       string             `json:"image"`
	State       cri.ContainerState `json:"state"`
	Labels      map[string]string  `json:"labels"`
	Annotations map[string]string  `json:"annotations"`
}

func isPathAllowed(containerPath string) bool {
	// Check if this container path is explicitly allowed.
	for _, allowedPath := range containerPathAllowList {
		if isInsidePath(containerPath, allowedPath) {
			return true
		}
	}

	// Check if this container path is explicitly disallowed.
	for _, disallowedPath := range containerPathDisallowList {
		if isInsidePath(containerPath, disallowedPath) {
			return false
		}
	}

	return true
}

func makeContainerKey(key string) string {
	return filepath.Join(containerSubdir, fmt.Sprintf("%s%s", containerPrefix, key))
}

func (rs *RuntimeService) getContainer(key string) *Container {
	key = makeContainerKey(key)

	buf, err := rs.dataStore.Read(key)
	if err != nil {
		klog.Errorf("looking up %s: %v", key, err)
		return nil
	}

	data := Container{}
	err = json.Unmarshal(buf, &data)
	if err != nil {
		klog.Errorf("deserializing data for %s: %v", key, err)
		return nil
	}

	return &data
}

func (rs *RuntimeService) putContainer(key string, data *Container) {
	key = makeContainerKey(key)

	buf, err := json.Marshal(data)
	if err != nil {
		klog.Errorf("serializing data for %s: %v", key, err)
		return
	}

	if err := rs.dataStore.Write(key, buf); err != nil {
		klog.Errorf("storing %s: %v", key, err)
		return
	}
}

func (rs *RuntimeService) deleteContainer(key string) bool {
	key = makeContainerKey(key)

	err := rs.dataStore.Erase(key)
	if err != nil {
		klog.Errorf("deleting %s: %v", key, err)
		return false
	}

	return true
}

func (rs *RuntimeService) listContainers() []*Container {
	list := make([]*Container, 0)

	for key := range rs.dataStore.Keys(nil) {
		if !strings.HasPrefix(key, containerPrefix) {
			continue
		}

		key = strings.Replace(key, containerPrefix, "", 1)

		entry := rs.getContainer(key)
		if entry != nil {
			list = append(list, entry)
		}
	}
	return list
}

func makeEnvList(envs []*cri.KeyValue) []string {
	hostname, err := os.Hostname()
	if err != nil {
		klog.Warningf("Hostname(): %v", err)
	}

	defaultEnvMap := make(map[string]string)
	defaultEnvMap["HOSTNAME"] = hostname
	defaultEnvMap["TERM"] = "xterm"
	defaultEnvMap["HOME"] = "/"
	defaultEnvMap["PATH"] = defaultPath

	ret := make([]string, 0, len(envs))

	for _, kv := range envs {
		delete(defaultEnvMap, kv.Key)
		ret = append(ret, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}

	for k, v := range defaultEnvMap {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}

	klog.V(5).Infof("created env %v", ret)
	return ret
}

//
// Implementation of container calls in cri.Runtimeservice.
//

func symlinkToContainerPath(hostPath, containerPath string) error {
	if !isPathAllowed(containerPath) {
		klog.Warningf("mount %s->%s is not allowed", hostPath, containerPath)
		return nil
	}

	// This will symlink any volume mount from its host path to the path
	// where the pod expects it to be. Note: this will overwrite files or
	// empty directories at the host path (but not host path directories
	// that are not empty).
	if err := os.Remove(containerPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(containerPath), 0755); err != nil {
		return err
	}
	if err := os.Symlink(hostPath, containerPath); err != nil {
		return err
	}
	klog.V(3).Infof("symlinked volume mount %s->%s", hostPath, containerPath)
	return nil
}

// CreateContainer creates a new container in specified PodSandbox
func (rs *RuntimeService) CreateContainer(ctx context.Context, req *cri.CreateContainerRequest) (*cri.CreateContainerResponse, error) {
	klog.V(4).Infof("CreateContainer config %+v", req.Config)

	// Required parameters.
	if req.Config == nil {
		klog.Errorf("CreateContainer: nil config")
		return nil, InvalidParameterError("CreateContainerRequest.Config")
	}
	if req.Config.Metadata == nil {
		klog.Errorf("CreateContainer: nil metadata")
		return nil, InvalidParameterError("CreateContainerRequest.Config.Metadata")
	}
	if req.Config.Image == nil {
		klog.Errorf("CreateContainer: nil image")
		return nil, InvalidParameterError("CreateContainerRequest.Config.Image")
	}
	if req.SandboxConfig == nil || req.SandboxConfig.Metadata == nil {
		klog.Errorf("CreateContainer: nil sandbox config or metadata")
		return nil, InvalidParameterError("CreateContainerRequest.SandboxConfig")
	}

	name := req.Config.Metadata.Name
	klog.V(2).Infof("CreateContainer %s", name)

	cid := xid.New().String()

	for _, m := range req.Config.Mounts {
		klog.V(5).Infof("CreateContainer %s %s -> %s", cid, m.HostPath, m.ContainerPath)
		if m.HostPath == m.ContainerPath {
			continue
		}
		err := symlinkToContainerPath(m.HostPath, m.ContainerPath)
		if err != nil {
			return nil, SymlinkError(err.Error())
		}
	}

	sandboxMetadata := req.SandboxConfig.Metadata
	podID := makePodID(sandboxMetadata.Namespace, sandboxMetadata.Name)

	pod := rs.getSandbox(podID)
	if pod == nil {
		err := fmt.Errorf("CreateContainer: no such pod sandbox %s", podID)
		klog.Error(err)
		return nil, InvalidParameterError(err.Error())
	}

	if pod.State != cri.PodSandboxState_SANDBOX_READY {
		err := fmt.Errorf("CreateContainer sandbox %s is not ready", podID)
		klog.Error(err)
		return nil, InvalidParameterError(err.Error())
	}

	pod.Containers = append(pod.Containers, cid)

	logPath := ""
	if req.Config.LogPath != "" {
		logPath = filepath.Join(pod.LogDirectory, req.Config.LogPath)
	}

	container := Container{
		ID:          cid,
		PodID:       podID,
		CreatedAt:   time.Now().UnixNano(),
		Name:        req.Config.Metadata.Name,
		Attempt:     req.Config.Metadata.Attempt,
		Image:       req.Config.Image.Image,
		Args:        req.Config.Args,
		Command:     req.Config.Command,
		WorkingDir:  req.Config.WorkingDir,
		LogPath:     logPath,
		Env:         makeEnvList(req.Config.Envs),
		State:       cri.ContainerState_CONTAINER_CREATED,
		Labels:      req.Config.Labels,
		Annotations: req.Config.Annotations,
	}
	rs.putContainer(cid, &container)

	rs.putSandbox(podID, pod)

	klog.V(2).Infof("CreateContainer: created container %s", cid)
	klog.V(5).Infof("CreateContainer LogPath: %s %s", container.LogPath, req.Config.LogPath)

	return &cri.CreateContainerResponse{
		ContainerId: cid,
	}, nil
}

func (rs *RuntimeService) trackContainerProcess(containerID string, cmd *exec.Cmd, lp *LogPipe, tty *os.File) {
	defer tty.Close()

	pid := cmd.Process.Pid

	klog.V(5).Infof("trackContainerProcess() waiting for logs of %d to finish", pid)
	lp.Wait()
	klog.V(5).Infof("trackContainerProcess() logs of %d finished", pid)

	if err := cmd.Wait(); err != nil {
		klog.Warningf("trackContainerProcess() waiting for container %s process %d: %v", containerID, pid, err)
	}

	cnt := rs.getContainer(containerID)
	if cnt == nil {
		klog.Errorf("trackContainerProcess() failed to get container %s", containerID)
		return
	}

	if cnt.Pid != pid {
		klog.Errorf("trackContainerProcess() container %s process ID changed %d -> %d", containerID, pid, cnt.Pid)
		return
	}

	ps := cmd.ProcessState
	exitCode := int32(ps.ExitCode())
	klog.V(5).Infof("trackContainerProcess() %s/%d exited: %d (%s); usr %v sys %v",
		containerID, pid, exitCode, ps.String(), ps.UserTime(), ps.SystemTime())

	cnt.ExitCode = exitCode
	cnt.State = cri.ContainerState_CONTAINER_EXITED
	rs.putContainer(containerID, cnt)
}

// StartContainer starts the container.
func (rs *RuntimeService) StartContainer(ctx context.Context, req *cri.StartContainerRequest) (*cri.StartContainerResponse, error) {
	klog.V(4).Infof("StartContainer request %+v", req)

	cid := req.ContainerId
	container := rs.getContainer(cid)
	if container == nil {
		klog.V(2).Infof("StartContainer %s: not found", cid)
		return nil, fmt.Errorf("container %s not found", cid)
	}
	commandArgs := container.Command
	commandArgs = append(commandArgs, container.Args...)
	if len(commandArgs) == 0 {
		klog.V(2).Infof("StartContainer %s: no command", cid)
		return nil, fmt.Errorf("container %s no command or args", cid)
	}

	if container.LogPath == "" {
		container.LogPath = "/tmp/cnt.log"
	}
	err := os.MkdirAll(filepath.Dir(container.LogPath), 0755)
	if err != nil {
		klog.Warningf("creating directory for logfile %s for %s: %v", container.LogPath, cid, err)
	}
	klog.V(5).Infof("StartContainer %s LogPath: %s", cid, container.LogPath)

	// Don't use Setpgid, it will fail since pty sets the new process as a session leader.
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = container.Env
	cmd.Dir = container.WorkingDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		klog.Errorf("StartContainer %s: cmd.StdoutPipe() %v", cid, err)
		return nil, fmt.Errorf("container %s start failed: %s", cid, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		klog.Errorf("StartContainer %s: cmd.StderrPipe() %v", cid, err)
		return nil, fmt.Errorf("container %s start failed: %s", cid, err)
	}

	tty, err := pty.Start(cmd)
	if err != nil {
		klog.Errorf("StartContainer %s: %v", cid, err)
		return nil, fmt.Errorf("container %s start failed: %s", cid, err)
	}

	lp, err := NewLogPipe(stdout, stderr, container.LogPath)
	if err != nil {
		klog.Errorf("StartContainer %s: NewLogPipe %v", cid, err)
		return nil, fmt.Errorf("container %s start failed: %s", cid, err)
	}
	lp.Start()

	container.Pid = cmd.Process.Pid
	container.State = cri.ContainerState_CONTAINER_RUNNING
	container.ExitCode = 0
	container.StartedAt = time.Now().UnixNano()

	rs.putContainer(cid, container)

	go rs.trackContainerProcess(container.ID, cmd, lp, tty)

	klog.V(2).Infof("StartContainer %s (%s) succeeded", cid, cmd.Path)
	return &cri.StartContainerResponse{}, nil
}

func (rs *RuntimeService) terminateContainer(ctx context.Context, container *Container, timeout int64) error {
	cid := container.ID

	if container.State != cri.ContainerState_CONTAINER_RUNNING {
		klog.V(2).Infof("container %s not running", cid)
		return nil
	}

	if container.Pid == 0 {
		err := fmt.Errorf("container %s not started", cid)
		klog.Errorf("%v", err)
		return err
	}

	proc, err := os.FindProcess(container.Pid)
	if err != nil {
		klog.Warningf("cannot find container %s main pid %d assuming that process exited", cid, container.Pid)
		return nil
	}
	pidToKill := container.Pid

	pgID, err := syscall.Getpgid(container.Pid)
	if err != nil {
		klog.Warningf("can't find container %s process group %d", cid, container.Pid)
	} else {
		pidToKill = -pgID
	}

	err = syscall.Kill(pidToKill, syscall.SIGTERM)
	if err != nil {
		klog.Warningf("trying to gracefully stop container %s process %d: %v", cid, pidToKill, err)
		_ = syscall.Kill(pidToKill, syscall.SIGKILL)
		return nil
	}

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			container := rs.getContainer(cid)
			if container == nil {
				klog.Errorf("terminating container %s process %d: already removed?", cid, pidToKill)
				return nil
			}
			if container.State != cri.ContainerState_CONTAINER_RUNNING {
				klog.V(5).Infof("exit code for container %s process %d: %d", cid, container.Pid, container.ExitCode)
				return nil
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			klog.Warningf("timeout waiting for container %s process %d", cid, container.Pid)
			_ = proc.Signal(syscall.SIGKILL)
			return nil
		case <-ctx.Done():
			err = ctx.Err()
			klog.Warningf("waiting for container %s process %d: %v", cid, container.Pid, err)
			_ = proc.Signal(syscall.SIGKILL)
			return err
		}
	}
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
func (rs *RuntimeService) StopContainer(ctx context.Context, req *cri.StopContainerRequest) (*cri.StopContainerResponse, error) {
	klog.V(4).Infof("StopContainer request %+v", req)

	cid := req.ContainerId
	container := rs.getContainer(cid)
	if container == nil {
		klog.Warningf("StopContainer: container %s not found", cid)
		return nil, fmt.Errorf("cannot find container %s", cid)
	}

	err := rs.terminateContainer(ctx, container, req.Timeout)
	if err != nil {
		klog.Errorf("StopContainer: rs.terminateContainer failed with %v", err)
		return nil, err
	}

	klog.V(2).Infof("StopContainer %s succeeded", cid)
	return &cri.StopContainerResponse{}, nil
}

// RemoveContainer removes the container. If the container is running, the
// container must be forcibly removed.
// This call is idempotent, and must not return an error if the container has
// already been removed.
func (rs *RuntimeService) RemoveContainer(ctx context.Context, req *cri.RemoveContainerRequest) (*cri.RemoveContainerResponse, error) {
	klog.V(4).Infof("RemoveContainer request %+v", req)

	cid := req.ContainerId
	container := rs.getContainer(cid)
	if container == nil {
		klog.Warningf("RemoveContainer: container %s not found", cid)
		return nil, nil
	}

	pod := rs.getSandbox(container.PodID)
	if pod != nil {
		for i, ctrID := range pod.Containers {
			if ctrID != cid {
				continue
			}
			pod.Containers = append(pod.Containers[:i], pod.Containers[i+1:]...)
			rs.putSandbox(container.PodID, pod)
			break
		}
	} else {
		klog.Warningf("RemoveContainer %s no pod %s found", cid, container.PodID)
	}

	err := rs.terminateContainer(ctx, container, 0)
	if err != nil {
		klog.Errorf("RemoveContainer %s terminating failed: %v", cid, err)
		return nil, err
	}
	rs.deleteContainer(cid)

	klog.V(2).Infof("RemoveContainer %s success", req.ContainerId)
	return &cri.RemoveContainerResponse{}, nil
}

func containerToCRIContainer(cnt *Container) *cri.Container {
	return &cri.Container{
		Id:           cnt.ID,
		PodSandboxId: cnt.PodID,
		CreatedAt:    cnt.CreatedAt,
		State:        cnt.State,
		Labels:       cnt.Labels,
		Annotations:  cnt.Annotations,
		Metadata: &cri.ContainerMetadata{
			Name:    cnt.Name,
			Attempt: cnt.Attempt,
		},
		Image: &cri.ImageSpec{
			Image: cnt.Image,
		},
		ImageRef: cnt.Image,
	}
}

// ListContainers lists all containers by filters.
func (rs *RuntimeService) ListContainers(ctx context.Context, req *cri.ListContainersRequest) (*cri.ListContainersResponse, error) {
	klog.V(4).Infof("ListContainers request %+v", req)

	filteredContainers := filterContainers(rs.listContainers(), req.Filter)
	result := make([]*cri.Container, 0, len(filteredContainers))
	for _, cnt := range filteredContainers {
		result = append(result, containerToCRIContainer(cnt))
	}
	klog.V(4).Infof("ListContainers request %v: containers %v (%d)", req, result, len(result))
	return &cri.ListContainersResponse{
		Containers: result,
	}, nil
}

// ContainerStatus returns status of the container. If the container is not
// present, returns an error.
func (rs *RuntimeService) ContainerStatus(ctx context.Context, req *cri.ContainerStatusRequest) (*cri.ContainerStatusResponse, error) {
	klog.V(4).Infof("ContainerStatus request %+v", req)

	cid := req.ContainerId
	container := rs.getContainer(cid)
	if container == nil {
		klog.Warningf("ContainerStatus: container %s not found", cid)
		return nil, fmt.Errorf("container %s not found", cid)
	}

	resp := cri.ContainerStatusResponse{
		Status: &cri.ContainerStatus{
			Id: cid,
			Metadata: &cri.ContainerMetadata{
				Name:    container.Name,
				Attempt: container.Attempt,
			},
			State:      container.State,
			CreatedAt:  container.CreatedAt,
			StartedAt:  container.StartedAt,
			FinishedAt: container.FinishedAt,
			ExitCode:   container.ExitCode,
			Image: &cri.ImageSpec{
				Image: container.Image,
			},
			ImageRef:    container.Image,
			Reason:      "",
			Message:     "",
			Labels:      container.Labels,
			Annotations: container.Annotations,
			Mounts:      make([]*cri.Mount, 0),
			LogPath:     container.LogPath,
		},
		Info: make(map[string]string),
	}

	klog.V(4).Infof("ContainerStatus %s: %+v", cid, resp)
	return &resp, nil
}

// UpdateContainerResources updates ContainerConfig of the container.
func (rs *RuntimeService) UpdateContainerResources(ctx context.Context, req *cri.UpdateContainerResourcesRequest) (*cri.UpdateContainerResourcesResponse, error) {
	klog.V(4).Infof("UpdateContainerResources %+v", req)

	// TODO: check if resource spec has changed.
	klog.V(4).Infof("UpdateContainerResources %s resource spec %#v",
		req.ContainerId, req.Linux)

	klog.V(4).Infof("UpdateContainerResources for %s succeeded", req.ContainerId)
	return &cri.UpdateContainerResourcesResponse{}, nil
}

func (rs *RuntimeService) ReopenContainerLog(ctx context.Context, req *cri.ReopenContainerLogRequest) (*cri.ReopenContainerLogResponse, error) {
	klog.V(4).Infof("ReopenContainerLog %+v", req)

	// TODO: supporting this is non-trivial if the container process is running.

	klog.Errorf("ReopenContainerLog %+v: not supported", req)

	return nil, fmt.Errorf("ReopenContainerLog is not supported")
}

func filterContainersByLabel(labels map[string]string, containers []*Container) []*Container {
	if len(labels) == 0 {
		return containers
	}

	ret := make([]*Container, 0)

	for _, cnt := range containers {
		matches := 0

		for k, v := range labels {
			for pk, pv := range cnt.Labels {
				if pk == k && pv == v {
					matches++
				}
			}
		}
		if matches == len(labels) {
			ret = append(ret, cnt)
		}
	}
	return ret
}

func filterContainersByState(state *cri.ContainerStateValue, containers []*Container) []*Container {
	if state == nil {
		return containers
	}

	result := make([]*Container, 0)
	for _, cnt := range containers {
		if cnt.State == state.State {
			result = append(result, cnt)
		}
	}

	return result
}

func filterContainersByPodSandboxID(sandboxID string, containers []*Container) []*Container {
	if sandboxID == "" {
		return containers
	}

	result := make([]*Container, 0)
	for _, cnt := range containers {
		if cnt.PodID == sandboxID {
			result = append(result, cnt)
		}
	}
	return result
}

func filterContainersByID(containerID string, containers []*Container) []*Container {
	if containerID == "" {
		return containers
	}
	result := make([]*Container, 0)
	for _, cnt := range containers {
		if cnt.ID == containerID {
			result = append(result, cnt)
		}
	}
	return result
}

func filterContainers(containers []*Container, filters *cri.ContainerFilter) []*Container {
	result := filterContainersByID(
		filters.Id, filterContainersByState(
			filters.State, filterContainersByLabel(
				filters.LabelSelector, filterContainersByPodSandboxID(
					filters.PodSandboxId, containers))))
	return result
}
