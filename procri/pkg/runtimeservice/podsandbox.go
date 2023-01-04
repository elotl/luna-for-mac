package runtimeservice

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
)

const (
	sandboxSubdir = "sandbox/"
	sandboxPrefix = "sb_"
)

type Sandbox struct {
	ID           string
	Name         string
	Namespace    string
	UID          string
	Attempt      uint32
	CreatedAt    int64
	Hostname     string
	LogDirectory string
	State        cri.PodSandboxState
	Labels       map[string]string
	Annotations  map[string]string
	Containers   []string
}

//
// Implementation of podsandbox calls in cri.Runtimeservice.
//

func makePodID(namespace, name string) string {
	return fmt.Sprintf("%s_%s", namespace, name)
}

func makeSandboxKey(key string) string {
	return filepath.Join(sandboxSubdir, fmt.Sprintf("%s%s", sandboxPrefix, key))
}

func (rs *RuntimeService) getSandbox(key string) *Sandbox {
	key = makeSandboxKey(key)

	buf, err := rs.dataStore.Read(key)
	if err != nil {
		klog.V(5).Infof("looking up %s: %v", key, err)
		return nil
	}

	data := Sandbox{}
	err = json.Unmarshal(buf, &data)
	if err != nil {
		klog.Errorf("deserializing data for %s: %v", key, err)
		return nil
	}

	return &data
}

func (rs *RuntimeService) putSandbox(key string, data *Sandbox) {
	key = makeSandboxKey(key)

	buf, err := json.Marshal(data)
	if err != nil {
		klog.Errorf("serializing data for %s: %v", key, err)
		return
	}

	err = rs.dataStore.Write(key, buf)
	if err != nil {
		klog.Errorf("storing %s: %v", key, err)
		return
	}
}

func (rs *RuntimeService) deleteSandbox(key string) bool {
	key = makeSandboxKey(key)

	err := rs.dataStore.Erase(key)
	if err != nil {
		klog.Errorf("deleting %s: %v", key, err)
		return false
	}

	return true
}

func (rs *RuntimeService) listSandboxes() []*Sandbox {
	list := make([]*Sandbox, 0)

	for key := range rs.dataStore.Keys(nil) {
		if !strings.HasPrefix(key, sandboxPrefix) {
			continue
		}

		key = strings.Replace(key, sandboxPrefix, "", 1)
		entry := rs.getSandbox(key)
		if entry != nil {
			list = append(list, entry)
		}
	}

	return list
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes must ensure
// the sandbox is in the ready state on success.
func (rs *RuntimeService) RunPodSandbox(ctx context.Context, req *cri.RunPodSandboxRequest) (*cri.RunPodSandboxResponse, error) {
	klog.V(4).Infof("RunPodSandbox request %+v", req)

	if req.Config == nil || req.Config.Metadata == nil {
		err := fmt.Errorf("PodSandbox missing configuration in %#v", req)
		klog.Errorf("%v", err)
		return nil, err
	}

	podID := makePodID(req.Config.Metadata.Namespace, req.Config.Metadata.Name)
	if rs.getSandbox(podID) != nil {
		err := fmt.Errorf("PodSandbox %s already exists", podID)
		klog.V(2).Infof("%v", err)
		return nil, err
	}

	sandbox := Sandbox{
		ID:           podID,
		Name:         req.Config.Metadata.Name,
		Namespace:    req.Config.Metadata.Namespace,
		UID:          req.Config.Metadata.Uid,
		Attempt:      req.Config.Metadata.Attempt,
		Hostname:     req.Config.Hostname,
		LogDirectory: req.Config.LogDirectory,
		State:        cri.PodSandboxState_SANDBOX_READY,
		Labels:       req.Config.Labels,
		Annotations:  req.Config.Annotations,
		CreatedAt:    time.Now().UnixNano(),
	}

	rs.putSandbox(podID, &sandbox)

	resp := cri.RunPodSandboxResponse{
		PodSandboxId: podID,
	}

	klog.V(4).Infof("RunPodSandbox: created %s", podID)
	return &resp, nil
}

func (rs *RuntimeService) terminateSandboxContainers(ctx context.Context, pod *Sandbox, force bool) error {
	podID := pod.ID

	containers := make([]string, len(pod.Containers))
	copy(containers, pod.Containers)

	for i, cntID := range containers {
		cnt := rs.getContainer(cntID)
		if cnt != nil {
			timeout := int64(30)
			if force {
				timeout = 0
			}

			err := rs.terminateContainer(ctx, cnt, timeout)
			if err != nil {
				klog.Errorf("%s deleting container %s: %v", podID, cntID, err)
				return err
			}
			rs.deleteContainer(cntID)
		}

		pod.Containers = containers[i+1:]
		rs.putSandbox(podID, pod)
	}

	pod.State = cri.PodSandboxState_SANDBOX_NOTREADY
	rs.putSandbox(podID, pod)

	return nil
}

func (rs *RuntimeService) removeSandbox(ctx context.Context, podID string) error {
	pod := rs.getSandbox(podID)
	if pod == nil {
		return nil
	}

	if err := rs.terminateSandboxContainers(ctx, pod, true); err != nil {
		return err
	}

	rs.deleteSandbox(podID)

	return nil
}

// StopPodSandbox stops any running process that is part of the sandbox and
// reclaims network resources (e.g., IP addresses) allocated to the sandbox.
// If there are any running containers in the sandbox, they must be forcibly
// terminated.
// This call is idempotent, and must not return an error if all relevant
// resources have already been reclaimed. kubelet will call StopPodSandbox
// at least once before calling RemovePodSandbox. It will also attempt to
// reclaim resources eagerly, as soon as a sandbox is not needed. Hence,
// multiple StopPodSandbox calls are expected.
func (rs *RuntimeService) StopPodSandbox(ctx context.Context, req *cri.StopPodSandboxRequest) (*cri.StopPodSandboxResponse, error) {
	klog.V(4).Infof("StopPodSandbox request %+v", req)

	resp := cri.StopPodSandboxResponse{}

	pod := rs.getSandbox(req.PodSandboxId)
	if pod == nil {
		klog.Errorf("StopPodSandbox: %s does not exist", req.PodSandboxId)
		// Don't return error if sandbox is not found.
		return &resp, nil
	}

	if err := rs.terminateSandboxContainers(ctx, pod, false); err != nil {
		klog.Errorf("StopPodSandbox terminateSandboxContainers err: %v", err)
		return nil, err
	}

	klog.V(4).Infof("StopPodSandbox for %s succeeded", req.PodSandboxId)
	return &resp, nil
}

// RemovePodSandbox removes the sandbox. If there are any running containers
// in the sandbox, they must be forcibly terminated and removed.
// This call is idempotent, and must not return an error if the sandbox has
// already been removed.
func (rs *RuntimeService) RemovePodSandbox(ctx context.Context, req *cri.RemovePodSandboxRequest) (*cri.RemovePodSandboxResponse, error) {
	klog.V(4).Infof("RemovePodSandbox request %+v", req)

	err := rs.removeSandbox(ctx, req.PodSandboxId)
	if err != nil {
		klog.Errorf("RemovePodSandbox error: %v", err)
		return nil, err
	}

	resp := cri.RemovePodSandboxResponse{}

	klog.V(4).Infof("RemovePodSandbox for %s succeeded", req.PodSandboxId)
	return &resp, nil
}

// PodSandboxStatus returns the status of the PodSandbox. If the PodSandbox is not
// present, returns an error.
func (rs *RuntimeService) PodSandboxStatus(ctx context.Context, req *cri.PodSandboxStatusRequest) (*cri.PodSandboxStatusResponse, error) {
	klog.V(4).Infof("PodSandboxStatus request %+v", req)

	podID := req.PodSandboxId

	pod := rs.getSandbox(podID)
	if pod == nil {
		klog.Errorf("PodSandboxStatus %s error: not found", podID)
		return nil, fmt.Errorf("not found")
	}

	resp := cri.PodSandboxStatusResponse{
		Status: &cri.PodSandboxStatus{
			Id: pod.UID,
			Metadata: &cri.PodSandboxMetadata{
				Uid:       pod.UID,
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Attempt:   pod.Attempt,
			},
			State:     pod.State,
			CreatedAt: pod.CreatedAt,
			Network: &cri.PodSandboxNetworkStatus{
				Ip: rs.ipAddress,
			},
			Linux:       &cri.LinuxPodSandboxStatus{},
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Info: make(map[string]string),
	}

	klog.V(4).Infof("PodSandboxStatus for %s: %v", podID, resp.Status.State)
	return &resp, nil
}

func filterPodsByName(podID string, pods []*Sandbox) []*Sandbox {
	if podID == "" {
		return pods
	}

	ret := make([]*Sandbox, 0)

	for _, pod := range pods {
		if podID == pod.ID {
			ret = append(ret, pod)
		}
	}

	return ret
}

func filterPodsByLabel(labels map[string]string, pods []*Sandbox) []*Sandbox {
	if len(labels) == 0 {
		return pods
	}

	ret := make([]*Sandbox, 0)

	for _, pod := range pods {
		matches := 0

		for k, v := range labels {
			for pk, pv := range pod.Labels {
				if pk == k && pv == v {
					matches++
				}
			}
		}

		if matches == len(labels) {
			ret = append(ret, pod)
		}
	}

	return ret
}

func filterPodsByState(state *cri.PodSandboxStateValue, pods []*Sandbox) []*Sandbox {
	if state == nil {
		return pods
	}

	result := make([]*Sandbox, 0)
	for i := range pods {
		if pods[i].State == state.State {
			result = append(result, pods[i])
		}
	}

	return result
}

// ListPodSandbox returns a list of PodSandboxes.
func (rs *RuntimeService) ListPodSandbox(ctx context.Context, req *cri.ListPodSandboxRequest) (*cri.ListPodSandboxResponse, error) {
	klog.V(4).Infof("ListPodSandbox request %+v", req)

	pods := rs.listSandboxes()
	if req.Filter != nil {
		pods = filterPodsByName(req.Filter.Id, pods)
		pods = filterPodsByLabel(req.Filter.LabelSelector, pods)
		pods = filterPodsByState(req.Filter.State, pods)
	}

	items := make([]*cri.PodSandbox, 0)
	names := make([]string, 0)
	for _, pod := range pods {
		sb := &cri.PodSandbox{
			Id: pod.ID,
			Metadata: &cri.PodSandboxMetadata{
				Uid:       pod.UID,
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Attempt:   pod.Attempt,
			},
			State:       pod.State,
			CreatedAt:   pod.CreatedAt,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		}
		items = append(items, sb)
		names = append(names, sb.Metadata.Name)
	}

	resp := cri.ListPodSandboxResponse{
		Items: items,
	}

	klog.V(4).Infof("ListPodSandbox request %v found %v", req, names)
	return &resp, nil
}
