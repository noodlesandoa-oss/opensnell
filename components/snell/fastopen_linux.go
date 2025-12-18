package snell

import (
	"errors"
	"net"
	"syscall"

	log "github.com/golang/glog"
)

const tcpFastOpenOpt = 23 // TCP_FASTOPEN on Linux

func setTcpFastOpen(lis net.Listener, enable int) error {
	if tl, ok := lis.(*net.TCPListener); ok {
		file, err := tl.File()
		if err != nil {
			return err
		}
		defer file.Close()
		sysconn, err := file.SyscallConn()
		if err != nil {
			return err
		}

		var setErr error
		ctrlErr := sysconn.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, tcpFastOpenOpt, enable)
		})
		if ctrlErr != nil {
			return ctrlErr
		}
		if setErr != nil {
			log.Warningf("failed to set TCP fastopen: %v\n", setErr)
			return setErr
		}
		log.Infof("TCP fastopen enabled=%d on %s\n", enable, lis.Addr().String())
		return nil
	}
	return errors.New("invalid listener")
}
