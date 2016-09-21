package service

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"imagecatcher/config"
	"imagecatcher/daotest"
)

var (
	benchTCPHandler  ServerHandler
	benchTCPListener net.Listener
)

func init() {
	rand.Seed(time.Now().UnixNano())
	daotest.InitTestDB()
	testConfig := config.Config{
		"TILE_PROCESSING_WORKERS":    8,
		"TILE_PROCESSING_QUEUE_SIZE": 8,
		"CONTENT_STORE_WORKERS":      8,
		"CONTENT_STORE_QUEUE_SIZE":   8,
	}
	jfsService := createJfsService()
	s := &imageCatcherServiceImpl{
		config:    testConfig,
		dbHandler: daotest.TestDBHandler,
	}
	s.initializeJFS(fakeJfsFactory{jfsService})

	benchTCPRequestHandler := NewTileRequestHandler(s, NewEmailNotifier("tcp test", testConfig), testConfig)
	importAcquisition(s)
	benchTCPHandler, benchTCPListener = prepareTestTCPServer(benchTCPRequestHandler)
	go startListening(benchTCPHandler)
}

func BenchmarkDbTCPSend5MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			resp, _ := tcpSendTile(benchTCPListener.Addr().String(), testAcqID, testCamera, testCol, testRow, smallImage)
			if resp.Status != 0 {
				b.Skip()
			}
		}
	})
}

func BenchmarkDbTCPSend20MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			tcpSendTile(benchTCPListener.Addr().String(), testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}
