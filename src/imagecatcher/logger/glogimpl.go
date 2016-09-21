package logger

import (
	"flag"
	"github.com/golang/glog"
	"log"
	"os"
	"strconv"
)

type gLogger struct {
	level int
}

func init() {
	currentLogger = gLogger{ErrorLevel}
}

// SetupLogger initialize the logger
func SetupLogger(maxSize int, level int) {
	if maxSize > 0 {
		glog.MaxSize = uint64(maxSize * 1024 * 1024)
	}
	// handle log_dir argument
	logDirArg := flag.Lookup("log_dir")
	if logDirArg.Value.String() != "" {
		if logDir := logDirArg.Value.String(); logDir != "." && logDir != ".." {
			err := os.MkdirAll(logDir, os.ModePerm)
			if err != nil {
				log.Print(err)
			}
		}
	}
	// handle v argument
	vArg := flag.Lookup("v")
	loggerLevel := level
	if vArg.Value.String() != "0" {
		var err error
		if loggerLevel, err = strconv.Atoi(vArg.Value.String()); err != nil {
			log.Print(err)
		}
	} else {
		// the command line arg is not set use the passed in parameter
		vArg.Value.Set(strconv.Itoa(loggerLevel))
	}
	glog.CopyStandardLogTo("INFO")
	currentLogger = gLogger{loggerLevel}
}

func (l gLogger) Logf(level int, format string, args ...interface{}) {
	switch level {
	case FatalLevel:
		glog.Fatalf(format, args...)
	case ErrorLevel:
		glog.Errorf(format, args...)
	case InfoLevel:
		glog.Infof(format, args...)
	case DebugLevel:
		glog.V(3).Infof(format, args...)
	case TraceLevel:
		glog.V(4).Infof(format, args...)
	default:
		glog.V(5).Infof(format, args...)
	}
}

func (l gLogger) Log(level int, args ...interface{}) {
	switch level {
	case FatalLevel:
		glog.Fatal(args...)
	case ErrorLevel:
		glog.Error(args...)
	case InfoLevel:
		glog.Info(args...)
	case DebugLevel:
		glog.V(2).Info(args...)
	case TraceLevel:
		glog.V(3).Info(args...)
	default:
		glog.V(4).Info(args...)
	}
}
