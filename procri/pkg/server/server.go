package server

import (
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/elotl/procri/pkg/imageservice"
	"github.com/elotl/procri/pkg/runtimeservice"
	"github.com/peterbourgon/diskv"

	"google.golang.org/grpc"
	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
	k8sstreaming "k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

type ProcriServer struct {
	listener       net.Listener
	server         *grpc.Server
	runtimeService *runtimeservice.RuntimeService
	imageService   *imageservice.ImageService
}

func NewServer(
	streamingServer k8sstreaming.Server,
	ipAddress string,
	dataStoreBasePath string,
	runtimeVersion string,
) (*ProcriServer, error) {
	imageDataStorePath := filepath.Join(dataStoreBasePath, "imageservice")
	imageDataStore := diskv.New(diskv.Options{BasePath: imageDataStorePath})
	imageService := imageservice.NewImageService(imageDataStore)

	runtimeDataStorePath := filepath.Join(dataStoreBasePath, "runtimeService")
	runtimeDataStore := diskv.New(diskv.Options{BasePath: runtimeDataStorePath})
	runtimeService, err := runtimeservice.NewRuntimeService(
		streamingServer,
		ipAddress,
		runtimeDataStore,
		runtimeVersion,
	)
	if err != nil {
		return nil, err
	}

	s := &ProcriServer{
		server:         grpc.NewServer(),
		imageService:   imageService,
		runtimeService: runtimeService,
	}
	cri.RegisterRuntimeServiceServer(s.server, s.runtimeService)
	cri.RegisterImageServiceServer(s.server, s.imageService)
	return s, nil
}

func (s *ProcriServer) Serve(addr string) error {
	klog.Infof("starting listener at %s", addr)
	if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
		return err
	}
	listener, err := net.Listen("unix", addr)
	if err != nil {
		klog.Fatalf("starting listener at %s: %v", addr, err)
	}
	s.listener = listener
	return s.server.Serve(listener)
}

func (s *ProcriServer) Close() error {
	s.server.Stop()
	return s.listener.Close()
}
