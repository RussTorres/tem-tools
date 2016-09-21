package netutils

import (
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"

	"imagecatcher/netutils/tuner"
)

const (
	unsupportedProtoError = "Only tcp4 and tcp6 are supported"
	filePrefix            = "port."
)

var listenerBacklog = maxListenerBacklog()

// Listen returns TCP listener with SO_REUSEPORT option set.
//
// Only tcp4 and tcp6 networks are supported.
//
func Listen(network, addr string, tuners ...tuner.Tuner) (l net.Listener, err error) {
	var (
		soType, fd int
		file       *os.File
		sockaddr   syscall.Sockaddr
	)

	if sockaddr, soType, err = getSockaddr(network, addr); err != nil {
		return nil, err
	}

	if fd, err = syscall.Socket(soType, syscall.SOCK_STREAM, syscall.IPPROTO_TCP); err != nil {
		return nil, err
	}

	if err = setDefaultListenerSockopts(fd); err != nil {
		closesocket(fd)
		return nil, err
	}

	defer func() {
		if err != nil {
			closesocket(fd)
		}
	}()

	for _, tune := range tuners {
		err = tune(fd)
		if err != nil {
			return nil, err
		}
	}

	if err = syscall.Bind(fd, sockaddr); err != nil {
		return nil, err
	}

	// Set backlog size to the maximum
	if err = syscall.Listen(fd, listenerBacklog); err != nil {
		return nil, err
	}

	// File Name get be nil
	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if l, err = net.FileListener(file); err != nil {
		return nil, err
	}

	if err = file.Close(); err != nil {
		return nil, err
	}

	return l, err
}

func getSockaddr(proto, addr string) (sa syscall.Sockaddr, soType int, err error) {
	var (
		addr4 [4]byte
		addr6 [16]byte
		ip    *net.TCPAddr
	)

	ip, err = net.ResolveTCPAddr(proto, addr)
	if err != nil {
		return nil, -1, err
	}
	switch proto {
	default:
		return nil, -1, errors.New(unsupportedProtoError)
	case "tcp":
		if ip.IP != nil {
			if ip.IP.To4() != nil {
				copy(addr4[:], ip.IP[12:16]) // copy last 4 bytes of slice to array
				return &syscall.SockaddrInet4{Port: ip.Port, Addr: addr4}, syscall.AF_INET, nil
			}
			copy(addr6[:], ip.IP) // copy all bytes of slice to array
		}
		return &syscall.SockaddrInet6{Port: ip.Port, Addr: addr6}, syscall.AF_INET6, nil
	case "tcp4":
		if ip.IP != nil {
			copy(addr4[:], ip.IP[12:16]) // copy last 4 bytes of slice to array
		}
		return &syscall.SockaddrInet4{Port: ip.Port, Addr: addr4}, syscall.AF_INET, nil
	case "tcp6":
		if ip.IP != nil {
			copy(addr6[:], ip.IP) // copy all bytes of slice to array
		}
		return &syscall.SockaddrInet6{Port: ip.Port, Addr: addr6}, syscall.AF_INET6, nil
	}
}

func closesocket(s int) error {
	return syscall.Close(s)
}

func setDefaultListenerSockopts(s int) error {
	// Allow reuse of recently-used addresses.
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1))
}
