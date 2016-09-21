package service

import (
	"math/rand"
	"net/http/httptest"
	"testing"
	"time"

	"imagecatcher/config"
	"imagecatcher/daotest"
)

var (
	serverWithRealService *httptest.Server
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

	importAcquisition(s)
	errNotifier := NewEmailNotifier("http test", testConfig)
	trh := NewTileRequestHandler(s, errNotifier, testConfig)
	testHandler := NewHTTPServerHandler(nil, s, trh, nil, nil, errNotifier, true)
	serverWithRealService = httptest.NewServer(testHandler.(*httpServerHandler).httpImpl.Handler)
}

func BenchmarkDbHTTPPostMP5MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpPostTile(serverWithRealService.URL, testAcqID, testCamera, testCol, testRow, smallImage)
		}
	})
}

func BenchmarkDbHTTPPostMP20MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpPostTile(serverWithRealService.URL, testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}

func BenchmarkDbHTTPPut5MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(serverWithRealService.URL, "PUT", testAcqID, testCamera, testCol, testRow, smallImage)
		}
	})
}

func BenchmarkDbHTTPPut20MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(serverWithRealService.URL, "PUT", testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}

func BenchmarkDbHTTPPostBody5MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(serverWithRealService.URL, "POST", testAcqID, testCamera, testCol, testRow, smallImage)
		}
	})
}

func BenchmarkDbHTTPPostBody20MOnlyCaptureImageRequests(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(serverWithRealService.URL, "POST", testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}
