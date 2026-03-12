package server

import (
	"errors"
	"io"
	"time"
)

var errDisconnect = errors.New("traffic logger requested disconnect")

type deadlineReadWriter interface {
	io.ReadWriter
	SetReadDeadline(t time.Time) error
}

func copyBufferLog(dst io.Writer, src io.Reader, timeout time.Duration, log func(n uint64) bool) error {
	buf := make([]byte, 32*1024)
	for {
		if timeout > 0 {
			if d, ok := src.(deadlineReadWriter); ok {
				_ = d.SetReadDeadline(time.Now().Add(timeout))
			}
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			if !log(uint64(nr)) {
				// Log returns false, which means that the client should be disconnected
				return errDisconnect
			}
			_, ew := dst.Write(buf[0:nr])
			if ew != nil {
				return ew
			}
		}
		if er != nil {
			if er == io.EOF {
				// EOF should not be considered as an error
				return nil
			}
			return er
		}
	}
}

func copyTwoWayWithLogger(id string, serverRw, remoteRw io.ReadWriter, timeout time.Duration, l TrafficLogger) error {
	errChan := make(chan error, 2)
	go func() {
		errChan <- copyBufferLog(serverRw, remoteRw, timeout, func(n uint64) bool {
			return l.LogTraffic(id, 0, n)
		})
	}()
	go func() {
		errChan <- copyBufferLog(remoteRw, serverRw, timeout, func(n uint64) bool {
			return l.LogTraffic(id, n, 0)
		})
	}()
	// Block until one of the two goroutines returns
	return <-errChan
}

// copyTwoWay is the "fast-path" version of copyTwoWayWithLogger that does not log traffic.
func copyTwoWay(serverRw, remoteRw io.ReadWriter, timeout time.Duration) error {
	errChan := make(chan error, 2)
	go func() {
		errChan <- copyBufferLog(serverRw, remoteRw, timeout, func(n uint64) bool { return true })
	}()
	go func() {
		errChan <- copyBufferLog(remoteRw, serverRw, timeout, func(n uint64) bool { return true })
	}()
	// Block until one of the two goroutines returns
	return <-errChan
}
