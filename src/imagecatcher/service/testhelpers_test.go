package service

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/JaneliaSciComp/janelia-file-services/JFSGolang/jfs"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"sync"
	"time"
)

const (
	testAcqLog            = "testdata/123_logfile.ini"
	testRoiSpec           = "testdata/123_ROI_spec.ini"
	testRoiTiles          = "testdata/123_ROI_tiles.csv"
	defaultStorageMaxSize = 512 * 1024
)

var (
	smallImage []byte
	largeImage []byte
)

func init() {
	rand.Seed(time.Now().UnixNano())
	smallImageSize := 5.3 * 1024 * 1024
	smallImage = createTestBuffer(int(smallImageSize))
	largeImage = createTestBuffer(20 * 1024 * 1024)
}

type localStorage struct {
	content   *list.List
	maxLength int
	lock      sync.RWMutex
}

func createLocalStorage() *localStorage {
	return &localStorage{content: list.New(), maxLength: defaultStorageMaxSize}
}

func (s *localStorage) add(item interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.content.Len() >= s.maxLength {
		s.content.Remove(s.content.Front())
	}
	s.content.PushBack(item)
}

func (s *localStorage) getLast() interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.content.Back().Value
}

type fakeJfsFactory struct {
	fakeJfsService jfs.FileService
}

func (f fakeJfsFactory) Initialize(username string, password string) error {
	return nil
}

func (f fakeJfsFactory) GetFileService(key string) jfs.FileService {
	return f.fakeJfsService
}

type fakeJfsService struct {
	jfs.FileService
	testStatusCode int
	storage        *localStorage
}

func createJfsService() *fakeJfsService {
	testJfsStore := createLocalStorage()
	testJfsService := &fakeJfsService{
		testStatusCode: 200,
		storage:        testJfsStore,
	}
	return testJfsService
}

func (jfs fakeJfsService) Open() error {
	return nil
}

func (jfs fakeJfsService) Close() {
	// do nothing
}

func (jfs fakeJfsService) Put(path string, content []byte, data map[string]string) (map[string]interface{}, error) {
	jfs.storage.add(FileParams{Name: path, Content: content})
	return map[string]interface{}{
		"scalityKey": "testKey",
		"jfsPath":    "testpath",
		"checksum":   "fedcba1234567890",
		"path":       path,
		"statusCode": jfs.testStatusCode,
	}, nil
}

func (jfs fakeJfsService) Verify(path string, params map[string]string) error {
	return nil
}

func createTestBuffer(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

func httpPostTile(serverURL string, acqID uint64, camera, col, row int, image []byte) (map[string]interface{}, error) {
	var tileInfo map[string]interface{}

	tileFileName := fmt.Sprintf("col%04d_row%04d_cam%d.tif", col, row, camera)
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d", serverURL, acqID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("tile-file", tileFileName)
	if err != nil {
		return tileInfo, fmt.Errorf("Error creating the request %s", err)
	}
	if _, err = io.Copy(part, bytes.NewReader(image)); err != nil {
		return tileInfo, err
	}
	if err = writer.WriteField("tile-filename", tileFileName); err != nil {
		return tileInfo, err
	}
	if err = writer.Close(); err != nil {
		return tileInfo, err
	}

	request, err := http.NewRequest("POST", storeTileEndpoint, body)
	if err != nil {
		return tileInfo, fmt.Errorf("Error creating the POST request for %s: %s", tileFileName, err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return tileInfo, fmt.Errorf("Error sending %s request: %s", tileFileName, err)
	}
	defer res.Body.Close()
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return tileInfo, fmt.Errorf("Error reading body response for %s: %s", tileFileName, err)
	}
	if err = json.Unmarshal(bodyBytes, &tileInfo); err != nil {
		return tileInfo, fmt.Errorf("Error decoding response for %s: %s", tileFileName, err)
	}
	if res.StatusCode != 200 {
		return tileInfo, fmt.Errorf("Http status code: %d", res.StatusCode)
	}
	return tileInfo, nil
}

func httpSendTile(serverURL, httpMethod string, acqID uint64, camera, col, row int, image []byte) (map[string]interface{}, string, error) {
	var tileInfo map[string]interface{}

	tileFileName := fmt.Sprintf("col%04d_row%04d_cam%d.tif", col, row, camera)
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d?tile-filename=%s", serverURL, acqID, tileFileName)

	request, err := http.NewRequest(httpMethod, storeTileEndpoint, bytes.NewReader(image))
	if err != nil {
		return tileInfo, tileFileName, fmt.Errorf("Error creating the PUT request for %s: %s", tileFileName, err)
	}
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return tileInfo, tileFileName, fmt.Errorf("Error sending %s request: %s", tileFileName, err)
	}
	defer res.Body.Close()
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return tileInfo, tileFileName, fmt.Errorf("Error reading body response for %s: %s", tileFileName, err)
	}
	if err = json.Unmarshal(bodyBytes, &tileInfo); err != nil {
		return tileInfo, tileFileName, fmt.Errorf("Error decoding response for %s: %s", tileFileName, err)
	}
	if res.StatusCode != 200 {
		return tileInfo, tileFileName, fmt.Errorf("Http status code: %d", res.StatusCode)
	}
	for k, v := range res.Header {
		tileInfo[k] = v
	}
	return tileInfo, tileFileName, nil
}
