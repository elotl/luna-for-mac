package runtimeservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/diskv"
	"golang.org/x/net/context"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	k8sstreaming "k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

func NotSupportedError(what string) error {
	return fmt.Errorf("Unsupported operation or parameter via CRI: %s", what)
}

func InvalidParameterError(what string) error {
	return fmt.Errorf("Invalid or missing parameter via CRI: %s", what)
}

func SymlinkError(what string) error {
	return fmt.Errorf("Symlink for container failed: %s", what)
}

type RuntimeService struct {
	streamingServer k8sstreaming.Server
	dataStore       *diskv.Diskv
	ipAddress       string
	runtimeVersion  string
}

func NewRuntimeService(
	streamingServer k8sstreaming.Server,
	ipAddress string,
	dataStore *diskv.Diskv,
	runtimeVersion string,
) (*RuntimeService, error) {
	err := os.MkdirAll(filepath.Join(dataStore.BasePath, sandboxSubdir), 0755)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Join(dataStore.BasePath, containerSubdir), 0755)
	if err != nil {
		return nil, err
	}
	return &RuntimeService{
		streamingServer: streamingServer,
		ipAddress:       ipAddress,
		dataStore:       dataStore,
		runtimeVersion:  runtimeVersion,
	}, nil
}

func convertToSemVer(buildVersion string) string {
	// buildVersion is either legit semver tag
	// or output of git describe --dirty (e.g. v0.0.1-12-gf102854-dirty)
	parts := strings.Split(buildVersion, "-")
	if len(parts) > 1 {
		return parts[0]
	}
	return buildVersion
}

// Version returns the runtime name, runtime version and runtime API version.
func (rs *RuntimeService) Version(ctx context.Context, req *cri.VersionRequest) (*cri.VersionResponse, error) {
	klog.V(4).Infof("Version request %+v", req)

	resp := cri.VersionResponse{
		Version:     req.Version, // Version of the kubelet runtime API.
		RuntimeName: "procri",    // Name of the container runtime.
		// These two must be semver-compatible.
		RuntimeVersion:    convertToSemVer(rs.runtimeVersion),
		RuntimeApiVersion: "0.0.0",
	}

	klog.V(4).Infof("Version request %v: %v", req, resp)
	return &resp, nil
}

// Status returns the status of the runtime.
func (rs *RuntimeService) Status(ctx context.Context, req *cri.StatusRequest) (*cri.StatusResponse, error) {
	klog.V(4).Infof("Status request %+v", req)

	runtimeCondition := cri.RuntimeCondition{
		Type:   cri.RuntimeReady,
		Status: true,
	}
	networkCondition := cri.RuntimeCondition{
		Type:   cri.NetworkReady,
		Status: true,
	}
	status := &cri.RuntimeStatus{
		Conditions: []*cri.RuntimeCondition{
			&runtimeCondition,
			&networkCondition,
		},
	}
	resp := cri.StatusResponse{
		Status: status,
	}

	klog.V(4).Infof("Status request %v: %v", req, resp)
	return &resp, nil
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (rs *RuntimeService) UpdateRuntimeConfig(ctx context.Context, req *cri.UpdateRuntimeConfigRequest) (*cri.UpdateRuntimeConfigResponse, error) {
	klog.V(4).Infof("UpdateRuntimeConfig request %+v", req)

	// CIDR to use for pod IP addresses.
	// req.RuntimeConfig.NetworkConfig.PodCidr
	resp := cri.UpdateRuntimeConfigResponse{}

	klog.V(4).Infof("UpdateRuntimeConfig request %v: %v", req, resp)
	return &resp, nil
}
