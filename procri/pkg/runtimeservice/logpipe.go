package runtimeservice

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"k8s.io/klog"
)

const (
	RFC3339NanoLenient = "2006-01-02T15:04:05.999999999Z07:00"
)

type LogPipe struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
	log    io.WriteCloser
	wg     *sync.WaitGroup
}

func NewLogPipe(stdout, stderr io.ReadCloser, logPath string) (*LogPipe, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &LogPipe{
		stdout: stdout,
		stderr: stderr,
		log:    logFile,
		wg:     &sync.WaitGroup{},
	}, nil
}

func (lp *LogPipe) Start() {
	lp.wg.Add(1)
	go func() {
		defer lp.wg.Done()
		pipeOutputToLogFile(lp.stdout, "stdout", lp.log)
	}()

	lp.wg.Add(1)
	go func() {
		defer lp.wg.Done()
		pipeOutputToLogFile(lp.stderr, "stderr", lp.log)
	}()
}

func (lp *LogPipe) Wait() {
	lp.wg.Wait()
}

func pipeOutputToLogFile(stream io.ReadCloser, streamType string, logFile io.Writer) {
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				klog.Errorf("reading %s from process: %s", streamType, err)
			} else {
				klog.V(5).Infof("EOF while reading %s from process", streamType)
			}
			break
		}
		timestamp := time.Now().Format(RFC3339NanoLenient)
		fmt.Fprintf(logFile, "%s %s F %s", timestamp, streamType, line)
	}
}
