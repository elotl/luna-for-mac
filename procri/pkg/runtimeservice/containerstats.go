package runtimeservice

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
)

//
// Implementation of containerstats calls in cri.Runtimeservice.
//

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (rs *RuntimeService) ContainerStats(ctx context.Context, req *cri.ContainerStatsRequest) (*cri.ContainerStatsResponse, error) {
	cid := req.ContainerId
	klog.V(2).Infof("ContainerStats %s", cid)

	cnt := rs.getContainer(cid)
	if cnt == nil {
		err := fmt.Errorf("ContainerStats %s: not found", cid)
		klog.Errorf("%v", err)
		return nil, err
	}

	timestamp := time.Now().UnixNano()
	stats := &cri.ContainerStats{
		Attributes: &cri.ContainerAttributes{
			Id: cid,
			Metadata: &cri.ContainerMetadata{
				Name:    cnt.Name,
				Attempt: cnt.Attempt,
			},
			Labels:      cnt.Labels,
			Annotations: cnt.Annotations,
		},
		// CPU usage gathered from the container.
		Cpu: &cri.CpuUsage{
			Timestamp: timestamp,
			UsageCoreNanoSeconds: &cri.UInt64Value{
				Value: uint64(0), // TODO
			},
		},
		// Memory usage gathered from the container.
		Memory: &cri.MemoryUsage{
			Timestamp: timestamp,
			WorkingSetBytes: &cri.UInt64Value{
				Value: uint64(0), // TODO
			},
		},
		// Usage of the writeable layer.
		WritableLayer: &cri.FilesystemUsage{
			Timestamp: timestamp,
			FsId: &cri.FilesystemIdentifier{
				Mountpoint: "/", // TODO
			},
			UsedBytes: &cri.UInt64Value{
				Value: uint64(0), // TODO
			},
			InodesUsed: &cri.UInt64Value{
				Value: uint64(0), // TODO
			},
		},
	}

	resp := &cri.ContainerStatsResponse{
		Stats: stats,
	}

	klog.V(2).Infof("ContainerStats %s: success", req.ContainerId)
	return resp, nil
}

// ListContainerStats returns stats of all running containers.
func (rs *RuntimeService) ListContainerStats(ctx context.Context, req *cri.ListContainerStatsRequest) (*cri.ListContainerStatsResponse, error) {
	klog.V(2).Infof("ListContainerStats %+v", req)

	lcr := &cri.ListContainersRequest{}
	if req.Filter != nil {
		lcr.Filter = &cri.ContainerFilter{
			Id:            req.Filter.Id,
			PodSandboxId:  req.Filter.PodSandboxId,
			LabelSelector: req.Filter.LabelSelector,
		}
	}
	resp, err := rs.ListContainers(ctx, lcr)
	if err != nil {
		return nil, err
	}

	lcsr := &cri.ListContainerStatsResponse{
		Stats: make([]*cri.ContainerStats, 0, len(resp.Containers)),
	}

	for _, c := range resp.Containers {
		csreq := &cri.ContainerStatsRequest{
			ContainerId: c.Id,
		}
		stats, err := rs.ContainerStats(ctx, csreq)
		if err != nil {
			klog.Warningf("ListContainerStats %s: %v", c.Id, err)
			continue
		}
		lcsr.Stats = append(lcsr.Stats, stats.Stats)
	}

	klog.V(2).Infof("ListContainerStats %+v: %d containers", req, len(lcsr.Stats))
	return lcsr, nil
}
