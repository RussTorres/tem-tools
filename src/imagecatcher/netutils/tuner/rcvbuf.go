package tuner

import (
	"syscall"

	"imagecatcher/logger"
)

func Rcvbuf(bufsize int) func(int) error {
	return func(sd int) error {
		if bufsize <= 0 {
			// ignore the setting
			logger.Printf("Using the default for the socket buffer size %d", bufsize)
			return nil
		}
		if err := syscall.SetsockoptInt(sd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, bufsize); err != nil {
			logger.Printf("Error setting receive buffer size: %s", err)
			return err
		}
		return nil
	}
}
