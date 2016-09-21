// +build darwin dragonfly freebsd netbsd openbsd

package tuner

import (
	"syscall"
)

const soReusePort = syscall.SO_REUSEPORT
