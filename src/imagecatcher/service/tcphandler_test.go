package service

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"testing"

	"imagecatcher/config"
	"imagecatcher/protocol"
)

func newLocalTCPListener() net.Listener {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			panic(fmt.Sprintf("tcptest: failed to listen on a port: %v", err))
		}
	}
	return l
}

func prepareTestTCPServer(reqHandler *TileRequestHandler) (ServerHandler, net.Listener) {
	tcpListener := newLocalTCPListener()
	server := NewTCPServerHandler(tcpListener, reqHandler)
	return server, tcpListener
}

func startListening(tcpHandler ServerHandler) {
	tcpHandler.Serve()
}

func setupTCP(testConfig config.Config) (*fakeImageCatcherService, ServerHandler, net.Listener) {
	serviceImpl := &fakeImageCatcherService{
		tileData:    createLocalStorage(),
		tileContent: createLocalStorage(),
	}
	tcpHandler, tcpListener := prepareTestTCPServer(NewTileRequestHandler(serviceImpl, createFakeErrorNotifier(), testConfig))
	return serviceImpl, tcpHandler, tcpListener
}

func TestTcpCaptureImageRequest(t *testing.T) {
	testConfig := config.Config{
		"TILE_PROCESSING_WORKERS":    1,
		"TILE_PROCESSING_QUEUE_SIZE": 1,
		"CONTENT_STORE_WORKERS":      1,
		"CONTENT_STORE_QUEUE_SIZE":   1,
	}
	fakeService, tcpHandler, tcpListener := setupTCP(testConfig)
	go startListening(tcpHandler)

	testBytes := createTestBuffer(testBufferLength)
	res, err := tcpSendTile(tcpListener.Addr().String(), testAcqID, testTileCamera, testTileCol, testTileRow, testBytes)
	if err != nil {
		t.Error(err)
	}
	if res.Status != 0 {
		t.Error("Expected status to be OK")
	}
	lastTileContent := fakeService.tileContent.getLast().(FileParams)
	if !bytes.Equal(lastTileContent.Content, testBytes) {
		t.Error("Expected tile content to be test content")
	}
}

func tcpSendTile(serverURL string, acqID uint64, camera, col, row int, image []byte) (*protocol.CaptureImageResponse, error) {
	tcpconn, err := net.Dial("tcp", serverURL)
	if err != nil {
		return nil, fmt.Errorf("Error opening the tcp connection to %s: %v", serverURL, err)
	}
	defer tcpconn.Close()

	checksum := md5.Sum(image)
	req := &protocol.CaptureImageRequest{
		AcqID:    acqID,
		Camera:   int32(camera),
		Frame:    -1,
		Col:      int32(col),
		Row:      int32(row),
		Image:    image,
		Checksum: checksum[0:],
	}
	tileReqBuf := protocol.MarshalTileRequest(req)
	if err := protocol.WriteUint32(tcpconn, uint32(len(tileReqBuf))); err != nil {
		return nil, fmt.Errorf("Error writing the total buffer length for %d:%d:%d:%d to %s: %v",
			req.AcqID, req.Camera, req.Col, req.Row, serverURL, err)
	}
	if _, err := tcpconn.Write(tileReqBuf); err != nil {
		return nil, fmt.Errorf("Error writing the buffer for %d:%d:%d:%d to %s: %v",
			req.AcqID, req.Camera, req.Col, req.Row, serverURL, err)
	}

	responseBufferLength, err := protocol.ReadUint32(tcpconn)
	if err != nil {
		return nil, fmt.Errorf("Error reading the response buffer length for %d:%d:%d:%d to %s: %v",
			req.AcqID, req.Camera, req.Col, req.Row, serverURL, err)
	}
	responseBuffer := bytes.NewBuffer(make([]byte, 0, responseBufferLength))
	n, err := responseBuffer.ReadFrom(io.LimitReader(tcpconn, int64(responseBufferLength)))
	if err != nil {
		return nil, fmt.Errorf("Error reading the response buffer for %d:%d:%d:%d to %s: %v",
			req.AcqID, req.Camera, req.Col, req.Row, serverURL, err)
	}
	if uint32(n) < responseBufferLength {
		return nil, fmt.Errorf("Expected to read %d bytes but instead it only read %d. This may result into a 'Unmarshalling error'!", responseBufferLength, n)
	}
	imageTileResponse := protocol.UnmarshalTileResponse(responseBuffer.Bytes())
	return imageTileResponse, nil
}
