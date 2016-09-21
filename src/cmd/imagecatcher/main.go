package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"sync"
	"syscall"

	"imagecatcher/config"
	"imagecatcher/dao"
	"imagecatcher/logger"
	"imagecatcher/netutils"
	"imagecatcher/netutils/tuner"
	"imagecatcher/service"
)

type stringArrayFlags []string

func (af *stringArrayFlags) String() string {
	return fmt.Sprint(*af)
}

func (af *stringArrayFlags) Set(value string) error {
	values := strings.Split(value, ",")
	for _, v := range values {
		*af = append(*af, strings.Trim(v, " "))
	}
	return nil
}

var (
	cpuprofile = flag.String("cpuprofile", "", "Name of the CPU profile file")
	memprofile = flag.String("memprofile", "", "Name of the memory profile file")
	tracefile  = flag.String("tracefile", "", "Name of the trace file")
	httpbind   = flag.String("httpserver", "", "HTTP server binding")
	tcpbind    = flag.String("tcpserver", "", "Raw TCP server binding")
)

func main() {
	var configs stringArrayFlags
	flag.Var(&configs, "config", "list configuration files")

	flag.Parse()
	if len(configs) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}
	startService(configs...)
}

func startService(configs ...string) {
	config, err := config.GetConfig(configs...)
	if err != nil {
		logger.Printf("Error reading the configuration(s): %v", err)
		return
	}

	logLevel := config.GetIntProperty("LOG_LEVEL", 0)
	logger.SetupLogger(config.GetIntProperty("LOG_MAX_SIZE", 0), logLevel)

	startProfiler()

	stopChan := make(chan os.Signal)
	go func() {
		<-stopChan
		stopProfiler()
		os.Exit(0)
	}()
	signal.Notify(stopChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGKILL)

	var dbHandler dao.DbHandler
	var imageCatcherService service.ImageCatcherService
	var wg sync.WaitGroup

	if dbHandler, err = dao.NewDbHandler(*config); err != nil {
		logger.Errorf("Error connecting to the database instance: %v", err)
		return
	}

	if imageCatcherService, err = service.NewService(dbHandler, *config); err != nil {
		logger.Errorf("Error instantiating the service: %v", err)
		return
	}

	serviceInstanceName, err := os.Hostname()
	if err != nil {
		logger.Errorf("Error retrieving sender host for the notifier: %v", err)
		serviceInstanceName = config.GetStringProperty("INSTANCE_NAME", "Unknown")
	}
	serviceInstanceName = fmt.Sprintf("%s (%s, %s)", serviceInstanceName, *httpbind, *tcpbind)

	notifier := service.NewEmailNotifier(serviceInstanceName, *config)
	tileRequestHandler := service.NewTileRequestHandler(imageCatcherService, notifier, *config)

	wg.Add(2)
	go startTCPServer(tileRequestHandler, *config, &wg)
	go startHTTPServer(imageCatcherService, tileRequestHandler, dbHandler, notifier, *config, &wg)
	wg.Wait()

	logger.Printf("ImageCatcher stopped")
}

func startHTTPServer(imageCatcherService service.ImageCatcherService,
	tileRequestHandler *service.TileRequestHandler,
	dbHandler dao.DbHandler,
	notifier service.MessageNotifier,
	config config.Config, wg *sync.WaitGroup) (err error) {
	defer wg.Done()
	if *httpbind == "" {
		return nil
	}

	var httpListener net.Listener
	if httpListener, err = netutils.Listen("tcp", *httpbind,
		tuner.Reuseport, tuner.Rcvbuf(config.GetIntProperty("SO_RECEIVE_BUFSIZE", 0))); err != nil {
		logger.Fatal(err)
	}
	defer httpListener.Close()

	tileDistributor := service.NewTileDistributor(dbHandler, config)
	configurator := service.NewConfigurator(dbHandler)
	keepAlives := !config.GetBoolProperty("DISABLE_SO_KEEP_ALIVE", false)
	httpServer := service.NewHTTPServerHandler(httpListener, imageCatcherService, tileRequestHandler, tileDistributor, configurator, notifier, keepAlives)

	logger.Printf("Starting HTTP ImageCatcher Service %s", *httpbind)
	if err = httpServer.Serve(); err != nil {
		logger.Errorf("Error starting the HTTP server: %v", err)
	}
	return err
}

func startTCPServer(tileRequestHandler *service.TileRequestHandler, config config.Config, wg *sync.WaitGroup) (err error) {
	defer wg.Done()
	if *tcpbind == "" {
		return nil
	}

	var tcpListener net.Listener
	if tcpListener, err = netutils.Listen("tcp", *tcpbind,
		tuner.Reuseport, tuner.Rcvbuf(config.GetIntProperty("SO_RECEIVE_BUFSIZE", 0))); err != nil {
		logger.Fatal(err)
	}
	defer tcpListener.Close()

	tcpServer := service.NewTCPServerHandler(tcpListener, tileRequestHandler)

	logger.Printf("Starting TCP ImageCatcher Service %s", *tcpbind)
	if err = tcpServer.Serve(); err != nil {
		logger.Printf("Error starting the TCP server: %v", err)
	}
	return err
}

func startProfiler() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			logger.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}
	if *tracefile != "" {
		f, err := os.Create(*tracefile)
		if err != nil {
			logger.Fatal(err)
		}
		trace.Start(f)
	}
}

func stopProfiler() {
	if *cpuprofile != "" {
		logger.Debugf("Storing CPU profiling to %s...\n", *cpuprofile)
		pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		logger.Debugf("Storing memory profiling to %s...\n", *memprofile)
		f, err := os.Create(*memprofile)
		if err != nil {
			logger.Print(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
	}
	if *tracefile != "" {
		trace.Stop()
	}
}
