package main

import (
	"flag"
	goflag "flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	k8sstreaming "k8s.io/kubernetes/pkg/kubelet/server/streaming"

	"github.com/spf13/pflag"

	"github.com/elotl/procri/pkg/server"
	"github.com/elotl/procri/pkg/streaming"

	k8snet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/klog"
)

var (
	BuildVersion = "N/A"
)

var (
	version           = pflag.Bool("version", false, "Print version and exit")
	streamingPort     = pflag.Int("streaming-port", 8099, "Port used for streaming")
	listen            = pflag.String("listen", "/var/run/procri.sock", "The sockets to listen on, e.g. /var/run/procri.sock")
	dataStoreBasePath = flag.String("data-store", "/tmp/procri-data.noindex", "directory for persisting data")
)

func main() {
	klogFlags := goflag.NewFlagSet(os.Args[0], goflag.ExitOnError)
	klog.InitFlags(klogFlags)

	pflag.CommandLine.AddGoFlagSet(klogFlags)
	if err := pflag.Set("logtostderr", "true"); err != nil {
		klog.Fatalf("%s", err)
	}
	pflag.Parse()

	if *version {
		klog.Infof("%s version: %s\n", os.Args[0], BuildVersion)
		os.Exit(0)
	}

	ipAddress, err := k8snet.ChooseHostInterface()
	if err != nil {
		klog.Fatalf("getting bind address for streaming server: %v", err)
	}
	hostAndPort := fmt.Sprintf("%s:%d", ipAddress.String(), *streamingPort)

	streamingServer, err := streaming.NewStreamingServer(hostAndPort)
	if err != nil {
		klog.Fatalf("creating streaming server: %v", err)
	}
	go func() {
		klog.Infof("starting streaming server on %s", hostAndPort)
		err = streamingServer.Start(true)
		if err != nil {
			klog.Fatalf("starting streaming server: %v", err)
		}
		defer func(streamingServer k8sstreaming.Server) {
			err := streamingServer.Stop()
			if err != nil {
				klog.Errorf("cannot close server %s", err)
			}
		}(streamingServer)
	}()

	klog.V(5).Infof("creating data store at base path %s", *dataStoreBasePath)
	err = os.MkdirAll(*dataStoreBasePath, 0755)
	if err != nil {
		klog.Fatalf("ensuring data store directory: %v", err)
	}

	klog.Infof("starting GRPC server")
	s, err := server.NewServer(streamingServer, ipAddress.String(), *dataStoreBasePath, BuildVersion)
	if err != nil {
		klog.Fatalf("creating server: %v", err)
	}
	defer func(s *server.ProcriServer) {
		err := s.Close()
		if err != nil {
			klog.Errorf("cannot close server %s", err)
		}
	}(s)

	if os.Getenv("PPROF_DEBUG") != "" {
		http.HandleFunc("/debug/pprof/", pprof.Index)
		http.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		http.HandleFunc("/debug/pprof/profile", pprof.Profile)
		http.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		http.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	err = s.Serve(*listen)
	if err != nil {
		klog.Fatalf("starting server: %v", err)
	}
}
