package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"imagecatcher/config"
	"imagecatcher/daotest"
	"imagecatcher/models"
)

const (
	testAcqID        = 123
	testTileCol      = 4321
	testTileRow      = 1234
	testTileCamera   = 2
	testBufferLength = 16384
)

func init() {
	rand.Seed(time.Now().UnixNano())
	daotest.InitTestDB()
}

func setupHTTP(messageNotifier MessageNotifier, td TileDistributor, testConfig config.Config) (*fakeImageCatcherService, *httptest.Server) {
	serviceImpl := &fakeImageCatcherService{
		tileData:    createLocalStorage(),
		tileContent: createLocalStorage(),
	}
	configurator := NewConfigurator(daotest.TestDBHandler)
	testHandler := NewHTTPServerHandler(nil, serviceImpl, NewTileRequestHandler(serviceImpl, messageNotifier, testConfig), td, configurator, messageNotifier, true)
	httpServer := httptest.NewServer(testHandler.(*httpServerHandler).httpImpl.Handler)
	return serviceImpl, httpServer
}

func setupWithContentChannelResult(messageNotifier MessageNotifier) (*fakeImageCatcherService, *TileRequestHandler, *httptest.Server) {
	testConfig := config.Config{
		"TILE_PROCESSING_WORKERS":    1,
		"TILE_PROCESSING_QUEUE_SIZE": 1,
		"CONTENT_STORE_WORKERS":      20,
		"CONTENT_STORE_QUEUE_SIZE":   20,
		"CONTENT_RESULT_BUFFER_SIZE": 1,
	}
	serviceImpl := &fakeImageCatcherService{
		tileData:    createLocalStorage(),
		tileContent: createLocalStorage(),
	}
	tileRequestHandler := NewTileRequestHandler(serviceImpl, messageNotifier, testConfig)
	testHandler := NewHTTPServerHandler(nil, serviceImpl, tileRequestHandler, nil, nil, messageNotifier, true)
	httpServer := httptest.NewServer(testHandler.(*httpServerHandler).httpImpl.Handler)
	return serviceImpl, tileRequestHandler, httpServer
}

func TestStartAcquisitionWithValidMPAcqLog(t *testing.T) {
	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/start-acquisition", httpServer.URL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("acq-inilog", testAcqLog)
	if err != nil {
		t.Error("Error writing acq-inilog part in the multipart form", testAcqLog)
		return
	}
	f, err := os.Open(testAcqLog)
	if err != nil {
		t.Error("Error encountered while trying to open", testAcqLog)
		return
	}
	if _, err = io.Copy(part, f); err != nil {
		t.Error("Error writing the multipart", testAcqLog)
		return
	}
	f.Close()
	err = writer.Close()
	if err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected a successful request but instead it got", res.StatusCode)
	}
	var reqResults map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if reqResults["errormessage"] != nil {
		t.Error("Unexpected error", reqResults["errormessage"])
		return
	}
	expectedResult := map[string]interface{}{
		"uid": 123,
	}
	if len(reqResults) != 1 && reqResults["uid"].(uint64) != 123 {
		t.Error("Expected result to be", expectedResult, "but got", reqResults)
	}
}

func writeFormFile(writer *multipart.Writer, fileName, fieldName string) error {
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return fmt.Errorf("Error writing %s:%s part", fieldName, fileName)
	}
	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("Error encountered while trying to open %s", fileName)
	}
	defer f.Close()
	if _, err = io.Copy(part, f); err != nil {
		return fmt.Errorf("Error writing %s:%s part", fieldName, fileName)
	}
	return nil
}

func TestStartAcquisitionWithEmptyNonMultipartForm(t *testing.T) {
	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/start-acquisition", httpServer.URL)

	body := &bytes.Buffer{}
	request, err := http.NewRequest("POST", url, body)

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 400 {
		t.Error("Expected invalid request but instead it got", res.StatusCode)
	}
	var reqResults map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if reqResults["errormessage"] == nil {
		t.Error("Expected an error message but it didn't get any")
		return
	}
	expectedMessage := "Error extracting the acquisition log content: request Content-Type isn't multipart/form-data"
	errorMessage := reqResults["errormessage"].(string)
	if errorMessage != expectedMessage {
		t.Error("Expected error message", expectedMessage, "but got", "'"+errorMessage+"'")
	}
}

func TestStartAcquisitionWithEmptyMultipartForm(t *testing.T) {
	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/start-acquisition", httpServer.URL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.Close()
	if err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType()) // make the request multi-part with empty body

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 400 {
		t.Error("Expected invalid request but instead it got", res.StatusCode)
	}
	var reqResults map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	if reqResults["errormessage"] == nil {
		t.Error("Expected an error message but it didn't get any")
		return
	}
	expectedMessage := "No acquisition log found in start-acquisition request"
	errorMessage := reqResults["errormessage"].(string)
	if errorMessage != expectedMessage {
		t.Error("Expected error message", expectedMessage, "but got", "'"+errorMessage+"'")
	}
}

func TestEndAcquisition(t *testing.T) {
	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/end-acquisition/123", httpServer.URL)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeFormFile(writer, testRoiTiles, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writeFormFile(writer, testRoiSpec, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writer.Close(); err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected a successful request but instead it got", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	expectedResult := `{"af_results":[` +
		`{"id":0,"jfs_key":"","jfs_path":"","name":"testdata/123_ROI_tiles.csv","path":""},` +
		`{"id":0,"jfs_key":"","jfs_path":"","name":"testdata/123_ROI_spec.ini","path":""}]}`
	if string(bodyBytes) != expectedResult {
		t.Error("Expected result to be", expectedResult, "but got", string(bodyBytes))
	}
}

func TestEndAcquisitionWithNoAncillaryFiles(t *testing.T) {
	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/end-acquisition/123", httpServer.URL)
	body := &bytes.Buffer{}

	request, err := http.NewRequest("POST", url, body)

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected a successful request but instead it got", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	expectedResult := "{}"
	if string(bodyBytes) != expectedResult {
		t.Error("Expected result to be", expectedResult, "but got", string(bodyBytes))
	}
}

func TestEndAcquisitionWithErrors(t *testing.T) {
	t.Parallel()
	emptyRoiSpec := "testdata/empty_ROI_spec.ini"
	emptyRoiTiles := "testdata/empty_ROI_tiles.csv"

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/end-acquisition/123", httpServer.URL)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeFormFile(writer, emptyRoiTiles, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writeFormFile(writer, emptyRoiSpec, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writer.Close(); err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 400 {
		t.Error("Expected an invalid request (400) but instead it got", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	expectedBody := `{"af_results":[` +
		`{"errormessage":"Empty content encountered for testdata/empty_ROI_tiles.csv","name":"testdata/empty_ROI_tiles.csv"},` +
		`{"errormessage":"Empty content encountered for testdata/empty_ROI_spec.ini","name":"testdata/empty_ROI_spec.ini"}]}`
	if string(bodyBytes) != expectedBody {
		t.Error("Expected result to be", expectedBody, "but got", string(bodyBytes))
	}
}

func TestHttpPostCaptureImageRequest(t *testing.T) {
	t.Parallel()
	fakeService, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	tileFileName := fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", testTileCol, testTileCol, testTileRow, testTileCamera)
	url := fmt.Sprintf("%s/service/v1/capture-image-content/%d", httpServer.URL, testAcqID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("tile-file", tileFileName)
	if err != nil {
		t.Error("Error creating the request", err)
	}

	testBytes := createTestBuffer(testBufferLength)
	_, err = io.Copy(part, bytes.NewReader(testBytes))

	err = writer.WriteField("tile-filename", tileFileName)
	if err != nil {
		t.Error("Error writing the tile filename field", err)
	}
	err = writer.Close()
	if err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Success expected but instead it got", res.StatusCode)
	}
	lastTileParams := fakeService.tileData.getLast().(TileParams)
	if lastTileParams.Col != testTileCol || lastTileParams.Row != testTileRow || lastTileParams.Camera != testTileCamera {
		t.Error("Expected",
			"col", testTileCol, "row", testTileRow, "camera", testTileCamera,
			"Got",
			"col", lastTileParams.Col, "row", lastTileParams.Row, "camera", lastTileParams.Camera)
	}
	lastTileContent := fakeService.tileContent.getLast().(FileParams)
	if !bytes.Equal(lastTileContent.Content, testBytes) {
		t.Error("Expected tile content to be test content")
	}
	if filepath.Base(tileFileName) != lastTileContent.Name {
		t.Error("Expected base name of", tileFileName, "got", lastTileContent.Name)
	}
	res.Body.Close()
}

func TestHttpPutCaptureImageRequest(t *testing.T) {
	t.Parallel()
	fakeService, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	testBytes := createTestBuffer(testBufferLength)
	res, tileFileName, err := httpSendTile(httpServer.URL, "PUT", testAcqID, testTileCamera, testTileCol, testTileRow, testBytes)
	if err != nil {
		t.Error(err)
	}
	lastTileParams := fakeService.tileData.getLast().(TileParams)
	if lastTileParams.Col != testTileCol ||
		lastTileParams.Row != testTileRow ||
		lastTileParams.Camera != testTileCamera ||
		lastTileParams.Frame != -1 {
		t.Error("Expected",
			"col", testTileCol, "row", testTileRow, "camera", testTileCamera,
			"Got",
			"col", lastTileParams.Col, "row", lastTileParams.Row, "camera", lastTileParams.Camera)
	}
	if int(res["tile_col"].(float64)) != testTileCol ||
		int(res["tile_row"].(float64)) != testTileRow ||
		int(res["tile_camera"].(float64)) != testTileCamera ||
		int(res["tile_frame"].(float64)) != -1 {
		t.Error("Expected",
			"col", testTileCol, "row", testTileRow, "camera", testTileCamera,
			"Got",
			"col", lastTileParams.Col, "row", lastTileParams.Row, "camera", lastTileParams.Camera)
	}

	lastTileContent := fakeService.tileContent.getLast().(FileParams)
	if !bytes.Equal(lastTileContent.Content, testBytes) {
		t.Error("Expected tile content to be test content")
	}
	if filepath.Base(tileFileName) != lastTileContent.Name {
		t.Error("Expected base name of", tileFileName, "got", lastTileContent.Name)
	}
}

func TestHttpPutWithErrorDuringTileProcessing(t *testing.T) {
	errNotifier := createFakeErrorNotifier()
	fakeService, httpServer := setupHTTP(errNotifier, nil, defaultTestingConfig)
	fakeService.tileData = nil // force a nil pointer dereference

	testBytes := createTestBuffer(testBufferLength)

	res, err := invokePutTileHTTPRequest(httpServer.URL, testBytes)
	defer res.Body.Close()
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 500 {
		t.Error("Expected status to be internal status error")
	}
	var lastErrMsg string
	lastErrMsg = errNotifier.messageStorage.getLast().(string)
	if strings.HasPrefix(lastErrMsg, "invalid memory address or nil pointer dereference") {
		t.Error("Expected a 'nil pointer dereference' error but got ", lastErrMsg)
	}
}

func TestHttpPutWithErrorDuringContentStorage(t *testing.T) {
	errNotifier := createFakeErrorNotifier()
	fakeService, tileRequestHandler, httpServer := setupWithContentChannelResult(errNotifier)
	fakeService.tileContent = nil

	testBytes := createTestBuffer(testBufferLength)

	res, err := invokePutTileHTTPRequest(httpServer.URL, testBytes)
	defer res.Body.Close()
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected status to be OK")
	}
	contentRes := <-tileRequestHandler.contentProcessedResBuffer
	var lastErrMsg string
	lastErrMsg = errNotifier.messageStorage.getLast().(string)
	if strings.HasPrefix(lastErrMsg, "invalid memory address or nil pointer dereference") {
		t.Error("Expected a 'nil pointer dereference' error but got ", lastErrMsg)
	}
	if contentRes.err.Error() != lastErrMsg {
		t.Error("Expected the content result and the error sent to be the same")
	}
}

func invokePutTileHTTPRequest(serverBaseURL string, tileBytes []byte) (*http.Response, error) {
	tileFileName := fmt.Sprintf("col%04d_row%04d_cam%d.tif", testTileCol, testTileRow, testTileCamera)
	captureTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d?tile-filename=%s", serverBaseURL, testAcqID, tileFileName)

	request, err := http.NewRequest("PUT", captureTileEndpoint, bytes.NewReader(tileBytes))
	if err != nil {
		return nil, fmt.Errorf("Error creating the put request %s - %v", captureTileEndpoint, err)
	}
	return http.DefaultClient.Do(request)
}

func TestHttpPutForQueueStatus(t *testing.T) {
	t.Parallel()
	testConfig := config.Config{
		"TILE_PROCESSING_WORKERS":    3,
		"TILE_PROCESSING_QUEUE_SIZE": 3,
		"CONTENT_STORE_WORKERS":      3,
		"CONTENT_STORE_QUEUE_SIZE":   3,
	}

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, testConfig)
	testBytes := createTestBuffer(testBufferLength)
	type putTileResponse struct {
		tileInfo map[string]interface{}
		err      error
	}
	var wg sync.WaitGroup
	nIterations := 75
	wg.Add(nIterations)
	responseChan := make(chan putTileResponse)
	for i := 0; i < nIterations; i++ {
		go func() {
			defer wg.Done()
			res, _, err := httpSendTile(httpServer.URL, "PUT", testAcqID, testTileCamera, i, testTileRow, testBytes)
			responseChan <- putTileResponse{res, err}
		}()
	}
	go func() {
		wg.Wait()
		close(responseChan)
	}()
	var nGreen, nYellow, nRed int
	for tr := range responseChan {
		if tr.err != nil {
			t.Error(tr.err)
		} else {
			switch tr.tileInfo["System Status"].([]string)[0] {
			case "GREEN":
				nGreen++
			case "YELLOW":
				nYellow++
			case "RED":
				nRed++
			}
		}
	}
	if nYellow == 0 && nRed == 0 {
		t.Error("Expected at least one red or yellow status",
			"Got instead", nYellow, "yellow", nRed, "red")
	}
	if nGreen == 0 {
		t.Error("Expected at least one green status")
	}
}

func TestStoreAncillaryFiles(t *testing.T) {
	t.Parallel()
	emptyRoiTiles := "testdata/empty_ROI_tiles.csv"

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/store-ancillary-files/123", httpServer.URL)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeFormFile(writer, emptyRoiTiles, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writeFormFile(writer, testRoiSpec, "ancillary-file"); err != nil {
		t.Error(err)
		return
	}
	if err := writer.Close(); err != nil {
		t.Error("Error closing the multipart writer", err)
	}

	request, err := http.NewRequest("POST", url, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 400 {
		t.Error("Expected an invalid request (400) but instead it got", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading start-acquisition response", err)
		return
	}
	expectedBody := `{"af_results":[` +
		`{"errormessage":"Empty content encountered for testdata/empty_ROI_tiles.csv","name":"testdata/empty_ROI_tiles.csv"},` +
		`{"id":0,"jfs_key":"","jfs_path":"","name":"testdata/123_ROI_spec.ini","path":""}]}`
	if string(bodyBytes) != expectedBody {
		t.Error("Expected result to be", expectedBody, "but got", string(bodyBytes))
	}
}

func TestImportCalibrations(t *testing.T) {
	const cfName = "testdata/141028_lens_correction_with_offset.json"

	t.Parallel()
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	bodyBytes, err := postCalibrations(httpServer.URL)
	if err != nil {
		t.Error(err)
		return
	}
	var respData map[string]interface{}
	if err = json.Unmarshal(bodyBytes, &respData); err != nil {
		t.Error("Error decoding response after posting", cfName)
		return
	}
	if respData["errormessage"] != nil {
		t.Error("Unexpected error", respData["errormessage"])
		return
	}
}

func postCalibrations(rootURL string) ([]byte, error) {
	const cfName = "testdata/141028_lens_correction_with_offset.json"

	url := fmt.Sprintf("%s/service/v1/calibrations", rootURL)
	cf, err := os.Open(cfName)
	if err != nil {
		return nil, err
	}
	defer cf.Close()
	request, err := http.NewRequest("POST", url, cf)
	if err != nil {
		return nil, fmt.Errorf("Error creating the request for posting %s to %s", cfName, url)
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Error posting the request for posting %s to %s", cfName, url)
	}
	defer res.Body.Close()
	respBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response after posting %s to %s", cfName, url)
	}
	if res.StatusCode != 200 {
		return respBytes, fmt.Errorf("Expected status 200 after posting %s to %s", cfName, url)
	}
	return respBytes, nil
}

func TestGetCalibrationWithValidName(t *testing.T) {
	const testCalibrationName = "141028offset_temca7_camera3"
	t.Parallel()

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	_, err := postCalibrations(httpServer.URL)

	url := fmt.Sprintf("%s/service/v1/calibration/name/%s", httpServer.URL, testCalibrationName)
	res, err := http.Get(url)
	if err != nil {
		t.Error("Error sending the GET request to", url)
		return
	}
	defer res.Body.Close()
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("Error reading calibration response from", url)
		return
	}

	var respData map[string]interface{}
	if err = json.Unmarshal(bodyBytes, &respData); err != nil {
		t.Error("Error decoding calibration response from", url)
		return
	}
	if res.StatusCode != 200 {
		t.Error("Expected status 200 from", url, "but it got", res.StatusCode)
		return
	}
	if respData["errormessage"] != nil {
		t.Error("Unexpected error", respData["errormessage"])
		return
	}
}

func TestGetCalibrationWithInvalidName(t *testing.T) {
	t.Parallel()

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	url := fmt.Sprintf("%s/service/v1/calibration/name/invalid", httpServer.URL)
	res, err := http.Get(url)
	if err != nil {
		t.Error("Error sending the GET request to", url)
		return
	}
	defer res.Body.Close()
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("Error reading calibration response from", url)
		return
	}

	if res.StatusCode != 404 {
		t.Error("Expected status 404 for", url, "but it got", res.StatusCode)
		return
	}
}

func TestGetProjects(t *testing.T) {
	t.Parallel()

	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)

	expectedResponseByURL := map[string]string{
		fmt.Sprintf("%s/service/v1/projects", httpServer.URL): `{` +
			`"length":3,` +
			`"projects":` +
			`[{"ProjectName":"p1.1","ProjectOwner":"o1"},{"ProjectName":"p1.2","ProjectOwner":"o1"},{"ProjectName":"p2.1","ProjectOwner":"o2"}]` +
			`}`,
		fmt.Sprintf("%s/service/v1/projects?owner=%s", httpServer.URL, "o1"): `{` +
			`"length":2,` +
			`"projects":` +
			`[{"ProjectName":"p1.1","ProjectOwner":"o1"},{"ProjectName":"p1.2","ProjectOwner":"o1"}]` +
			`}`,
		fmt.Sprintf("%s/service/v1/projects?project=%s&owner=%s", httpServer.URL, "p2.1", "o2"): `{` +
			`"length":1,` +
			`"projects":` +
			`[{"ProjectName":"p2.1","ProjectOwner":"o2"}]` +
			`}`,
		fmt.Sprintf("%s/service/v1/projects?project=%s&owner=%s", httpServer.URL, "p1.2", "o2"): `{` +
			`"length":0,` +
			`"projects":` +
			`[]` +
			`}`,
		fmt.Sprintf("%s/service/v1/projects?owner=%s", httpServer.URL, "invalid"): `{` +
			`"length":0,` +
			`"projects":` +
			`[]` +
			`}`,
	}
	for url, expectedResult := range expectedResponseByURL {
		res, err := http.Get(url)
		if err != nil {
			t.Error("Error sending the GET request to", url)
			return
		}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Error("Error reading response from", url)
			return
		}
		if res.StatusCode != 200 {
			t.Error("Expected status 200 for", url, "but it got", res.StatusCode)
			return
		}
		if strings.TrimSpace(string(bodyBytes)) != expectedResult {
			t.Error("Expected result to be", expectedResult, "but got", string(bodyBytes))
		}
	}
}

type testDistributor struct {
	tileData []*models.TileSpec
}

func (td *testDistributor) ServeNextAvailableTiles(tileFilter *models.TileFilter, newTileState string) ([]*models.TileSpec, SelectTileResultType, error) {
	var tilespecs []*models.TileSpec
	var tileResult SelectTileResultType
	if tileFilter.State == "" {
		return tilespecs, InvalidTileRequest, fmt.Errorf("Invalid tile request")
	}
	for i := 0; i < len(td.tileData); i++ {
		t := td.tileData[i]
		if t.State != tileFilter.State {
			continue
		}
		if tileFilter.AcqUID > 0 && t.AcqUID != tileFilter.AcqUID {
			continue
		}
		if tileFilter.NominalSection >= 0 && t.NominalSection != tileFilter.NominalSection {
			continue
		}
		tilespecs = append(tilespecs, t)
		t.State = newTileState
		tileResult = TileFound
		if tileFilter.Pagination.NRecords > 0 && len(tilespecs) >= int(tileFilter.Pagination.NRecords) {
			break
		}
	}
	if len(tilespecs) > 0 {
		return tilespecs, tileResult, nil
	}
	tileResult = NoTileReady
	if tileFilter.NominalSection > 0 {
		if tileFilter.NominalSection < 10 {
			tileResult = ServedEntireSection
		} else {
			tileResult = NoTileReadyInSection
		}
	} else if tileFilter.AcqUID > 0 {
		tileResult = ServedEntireAcq
	}
	return tilespecs, tileResult, nil
}

func (td *testDistributor) UpdateTilesState(tiles []*models.TileSpec, tileState string) (int64, error) {
	return 0, nil
}

func (td *testDistributor) stageCoord(t *models.TileSpec) (stageX float64, stageY float64) {
	switch t.NumberOfCameras {
	case 1:
		stageX = -(t.XCenter * 1.0E6 * t.PixPerUm) -
			(float64(t.XSmallStepPix) * float64(t.NXSteps) / 2.0) +
			float64(t.Col)*float64(t.XSmallStepPix)
		stageY = ((t.YCenter + t.YTem) * 1.0E6 * t.PixPerUm) -
			(float64(t.YSmallStepPix) * float64(t.NYSteps) / 2.0) +
			float64(t.Row)*float64(t.YSmallStepPix)
	case 4:
		stageX = -(t.XCenter * 1.0E6 * t.PixPerUm) -
			((float64(t.XBigStepPix) + float64(t.XSmallStepPix)) * (float64(t.NXSteps) / 4.0)) +
			((float64(t.XBigStepPix) + float64(t.XSmallStepPix)) * (float64(t.Col) / 4)) +
			(float64(t.XSmallStepPix) * float64(t.Col%4))
		stageY = ((t.YCenter + t.YTem) * 1.0E6 * t.PixPerUm) -
			((float64(t.YBigStepPix) + float64(t.YSmallStepPix)) * (float64(t.NYSteps) / 4.0)) +
			((float64(t.YBigStepPix) + float64(t.YSmallStepPix)) * (float64(t.Row) / 4)) +
			(float64(t.YSmallStepPix) * float64(t.Row%4))
	}
	return
}

func TestServeNextTile(t *testing.T) {
	var testAcqID uint64 = 456
	var testTiles = []*models.TileSpec{
		&models.TileSpec{
			Acquisition: models.Acquisition{
				AcqUID:          testAcqID,
				NumberOfCameras: 1,
				XSmallStepPix:   3946,
				YSmallStepPix:   3946,
				XBigStepPix:     3946,
				YBigStepPix:     3946,
				NXSteps:         22,
				NYSteps:         54,
				NTargetCols:     22,
				NTargetRows:     54,
				XCenter:         0.00025,
				YCenter:         0.001452,
				YTem:            0,
				PixPerUm:        242.1,
				Magnification:   2900,
			},
			Width:          1026,
			Height:         2026,
			NominalSection: 1,
			Col:            1,
			Row:            2,
			CameraConfig: models.TemCameraConfiguration{
				Camera:  0,
				TemcaID: 1,
			},
			ImageURL:            fmt.Sprintf("%d_%d_%d.tif", 1, 2, 0),
			MaskURL:             fmt.Sprintf("temca%d_cam%d", 1, 0),
			PrevAcqCount:        2,
			TransformationRefID: "TRef",
			State:               "READY",
		},
		&models.TileSpec{
			Acquisition: models.Acquisition{
				AcqUID:          testAcqID,
				NumberOfCameras: 4,
				XSmallStepPix:   1950,
				YSmallStepPix:   1850,
				XBigStepPix:     5550,
				YBigStepPix:     5450,
				NXSteps:         20,
				NYSteps:         84,
				NTargetCols:     40,
				NTargetRows:     168,
				XCenter:         0.000222953,
				YCenter:         -1.54074e-5,
				YTem:            -0.000255765,
				PixPerUm:        242,
				Magnification:   2900,
			},
			Width:          1026,
			Height:         2026,
			NominalSection: 2,
			Col:            2,
			Row:            3,
			CameraConfig: models.TemCameraConfiguration{
				Camera:  1,
				TemcaID: 2,
			},
			ImageURL:            fmt.Sprintf("%d_%d_%d.tif", 2, 3, 1),
			MaskURL:             fmt.Sprintf("temca%d_cam%d", 2, 1),
			PrevAcqCount:        3,
			TransformationRefID: "TRef",
			State:               "READY",
		},
		&models.TileSpec{
			Acquisition: models.Acquisition{
				AcqUID:          testAcqID,
				NumberOfCameras: 4,
				XSmallStepPix:   1950,
				YSmallStepPix:   1850,
				XBigStepPix:     5550,
				YBigStepPix:     5450,
				NXSteps:         20,
				NYSteps:         84,
				NTargetCols:     40,
				NTargetRows:     168,
				XCenter:         0.000222953,
				YCenter:         -1.54074e-5,
				YTem:            -0.000255765,
				PixPerUm:        242,
				Magnification:   2900,
			},
			Width:          1026,
			Height:         2026,
			NominalSection: 2,
			Col:            2,
			Row:            4,
			CameraConfig: models.TemCameraConfiguration{
				Camera:  1,
				TemcaID: 2,
			},
			ImageURL:            fmt.Sprintf("%d_%d_%d.tif", 2, 3, 1),
			MaskURL:             fmt.Sprintf("temca%d_cam%d", 2, 1),
			PrevAcqCount:        3,
			TransformationRefID: "TRef",
			State:               "READY",
		},
		&models.TileSpec{
			Acquisition: models.Acquisition{
				AcqUID:          testAcqID,
				NumberOfCameras: 4,
				XSmallStepPix:   1950,
				YSmallStepPix:   1850,
				XBigStepPix:     5550,
				YBigStepPix:     5450,
				NXSteps:         20,
				NYSteps:         84,
				NTargetCols:     40,
				NTargetRows:     168,
				XCenter:         0.000222953,
				YCenter:         -1.54074e-5,
				YTem:            -0.000255765,
				PixPerUm:        242,
				Magnification:   2900,
			},
			Width:          1026,
			Height:         2026,
			NominalSection: 2,
			Col:            2,
			Row:            5,
			CameraConfig: models.TemCameraConfiguration{
				Camera:  1,
				TemcaID: 2,
			},
			ImageURL:            fmt.Sprintf("%d_%d_%d.tif", 2, 3, 1),
			MaskURL:             fmt.Sprintf("temca%d_cam%d", 2, 1),
			PrevAcqCount:        3,
			TransformationRefID: "TRef",
			State:               "READY",
		},
	}
	td := &testDistributor{
		tileData: testTiles,
	}

	tests := []struct {
		reqAcqID       uint64
		reqSection     int64
		expectedResult SelectTileResultType
		expectedLen    int
		ntiles         int
		expectedTiles  []*models.TileSpec
	}{
		{0, -1, TileFound, 1, 1, []*models.TileSpec{testTiles[0]}},
		{testAcqID, -1, TileFound, 1, 1, []*models.TileSpec{testTiles[1]}},
		{testAcqID, 2, TileFound, 2, 2, []*models.TileSpec{testTiles[2], testTiles[3]}},
		{0, -1, NoTileReady, 0, 1, nil},
		{testAcqID, -1, ServedEntireAcq, 0, 1, nil},
		{testAcqID, 2, ServedEntireSection, 0, 1, nil},
		{testAcqID, 12, NoTileReadyInSection, 0, 1, nil},
	}

	_, httpServer := setupHTTP(createFakeErrorNotifier(), td, defaultTestingConfig)

	for i, testData := range tests {
		url := fmt.Sprintf("%s/service/v1/next-tile?acqid=%d&section=%d&ntiles=%d",
			httpServer.URL, testData.reqAcqID, testData.reqSection, testData.ntiles)
		bodyReader := strings.NewReader("")
		request, err := http.NewRequest("POST", url, bodyReader)
		if err != nil {
			t.Error("Test", i, url, "Unexpected error while creating the POST request for %s: %v", url, err)
		}
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err)
		}
		if res.StatusCode != 200 {
			t.Error("Test", i, url, "Expected status 200 but instead it got", res.StatusCode)
		}
		var reqResults map[string]interface{}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Error("Test", i, url, "Error reading update-tiles-state response", err)
			return
		}
		if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
			t.Error("Test", i, url, "Error reading update-tiles-state response", err, string(bodyBytes))
			return
		}
		if reqResults["errormessage"] != nil {
			t.Error("Test", i, url, "Didn't expect an error message but it got", reqResults["errormessage"])
			return
		}
		if reqResults["resultType"].(string) != testData.expectedResult.String() {
			t.Error("Test", i, url, "Expected result type to be", testData.expectedResult, "but it was", reqResults["resultType"])
		}
		if testData.reqAcqID > 0 && uint64(reqResults["requestedAcqId"].(float64)) != testData.reqAcqID {
			t.Error("Test", i, url, "Expected requested acqID to be set to", testData.reqAcqID, "but it was", reqResults["requestedAcqId"])
		}
		if testData.reqSection >= 0 && int64(reqResults["requestedSection"].(float64)) != testData.reqSection {
			t.Error("Test", i, url, "Expected requested section to be set to", testData.reqSection, "but it was", reqResults["requestedSection"])
		}
		if testData.expectedLen > 0 && (testData.expectedLen != len(reqResults["results"].([]interface{})) ||
			testData.expectedLen != int(reqResults["nTilesFound"].(float64))) {
			t.Error("Test", i, url, "Expected length", testData.expectedLen, "but it was", len(reqResults["results"].([]interface{})))
		}
		if testData.expectedLen == 1 && reqResults["tilespec"] != nil {
			t.Error("Test", i, url, "Did not expect tilespec to be set")
		}
		if testData.expectedLen == 1 && reqResults["acqid"] != nil {
			t.Error("Test", i, url, "Did not expect acqid to be set, but it found", reqResults["acqid"])
		}
		if testData.expectedLen == 1 && reqResults["section"] != nil {
			t.Error("Test", i, url, "Did not expected section to be set, but it was", reqResults["section"])
		}
		if testData.expectedLen == 1 && reqResults["results"] == nil {
			t.Error("Test", i, url, "Expect results to be set but it wasn't")
		}
		if testData.expectedLen > 1 && (reqResults["acqid"] != nil || reqResults["section"] != nil || reqResults["tilespec"] != nil) {
			t.Error("Test", i, url, "Didn't expect the top acqid and section to be set: ",
				reqResults["acqid"], reqResults["section"], reqResults["tilespec"])
		}
		if testData.expectedLen == 0 {
			if reqResults["results"] == nil {
				t.Error("Test", i, url, "Expect results to be set for empty results too but it wasn't")
				continue
			}
			results := reqResults["results"].([]interface{})
			if len(results) > 0 {
				t.Error("Test", i, url, "Expect results to be set an empty array but it had", len(results), "elements")
			}
			continue
		}
		for tileIndex, expectedTile := range testData.expectedTiles {
			tileResult := reqResults["results"].([]interface{})[tileIndex].(map[string]interface{})
			if uint64(tileResult["acqid"].(float64)) != expectedTile.AcqUID {
				t.Error("Test", i, "Expected acq ID to be", expectedTile.AcqUID, "but it was", tileResult["acqid"])
			}
			if int64(tileResult["section"].(float64)) != expectedTile.NominalSection {
				t.Error("Test", i, "Expected section to be", expectedTile.NominalSection, "but it was", tileResult["section"])
			}

			expectedTileID := fmt.Sprintf("%d.%03d.%03d.%d.%d",
				expectedTile.AcqUID, expectedTile.Col, expectedTile.Row, expectedTile.NominalSection, expectedTile.PrevAcqCount)
			tilespecResult := tileResult["tilespec"].(map[string]interface{})
			if tilespecResult["tileId"].(string) != expectedTileID {
				t.Error("Test", i, "Expected tileID to be", expectedTileID, "but it was", tilespecResult["tileId"])
			}

			stageX, stageY := td.stageCoord(expectedTile)
			sectionID := fmt.Sprintf("%d.%d", expectedTile.NominalSection, expectedTile.PrevAcqCount)
			expectedLayout := map[string]interface{}{
				"camera":    float64(expectedTile.CameraConfig.Camera),
				"imageCol":  float64(expectedTile.Col),
				"imageRow":  float64(expectedTile.Row),
				"sectionId": sectionID,
				"temca":     fmt.Sprintf("%d", expectedTile.CameraConfig.TemcaID),
				"stageX":    stageX,
				"stageY":    stageY,
				"rotation":  float64(0),
			}
			z, err := strconv.ParseFloat(sectionID, 64)
			const epsilon = 0.00000001
			if err != nil {
				t.Error("Test", i, "Unexpected error converting the section id to a float", sectionID, err)
			}
			if diff := math.Abs(z - tilespecResult["z"].(float64)); diff > epsilon {
				t.Error("Test", i, "Expected z to be", z, "but it was", tilespecResult["z"])
			}
			if !reflect.DeepEqual(expectedLayout, tilespecResult["layout"]) {
				t.Error("Test", i, "Expected layout to be", expectedLayout, "but it was", tilespecResult["layout"])
			}
			expectedTransforms := map[string]interface{}{
				"type": "list",
				"specList": []map[string]interface{}{
					map[string]interface{}{
						"type":  "ref",
						"refId": expectedTile.TransformationRefID,
					},
					map[string]interface{}{
						"type":       "leaf",
						"className":  "mpicbg.trakem2.transform.AffineModel2D",
						"dataString": fmt.Sprintf("1.0 0.0 0.0 1.0 %d %d", int(stageX), int(stageY)),
					},
				},
			}
			resultTransforms := tilespecResult["transforms"].(map[string]interface{})
			var jsonResultTransforms, jsonExpectedTransforms []byte
			if jsonResultTransforms, err = json.Marshal(resultTransforms); err != nil {
				t.Error("Unexpected error while encoding result transforms to json for comparison", resultTransforms, err)
			}
			if jsonExpectedTransforms, err = json.Marshal(expectedTransforms); err != nil {
				t.Error("Unexpected error while encoding expected transforms to json for comparison", expectedTransforms, err)
			}
			if string(jsonResultTransforms) != string(jsonExpectedTransforms) {
				t.Error("Test", i, "Expected transforms to be", expectedTransforms, "but it was", resultTransforms)
			}
		}
	}
}

func TestUpdateTilesState(t *testing.T) {
	td := &tileDistributorImpl{
		dbh: daotest.TestDBHandler,
		cfg: defaultTestingConfig,
	}
	_, httpServer := setupHTTP(createFakeErrorNotifier(), td, defaultTestingConfig)
	url := fmt.Sprintf("%s/service/v1/tile-state", httpServer.URL)

	bodyReader := strings.NewReader(`{
		"state": "NEW_STATE",
		"tileSpecIds": ["1.2.3.4.5", "1.2.6.7.9"]
	}`)
	request, err := http.NewRequest("PUT", url, bodyReader)
	if err != nil {
		t.Error("Unexpected error while creating the POST request", url, err)
	}
	request.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected status 200 but instead it got", res.StatusCode)
	}
	var reqResults map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Error("Error reading update-tiles-state response", err)
		return
	}
	if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
		t.Error("Error reading update-tiles-state response", err, string(bodyBytes))
		return
	}
	if reqResults["errormessage"] != nil {
		t.Error("Didn't expect an error message but it got ", reqResults["errormessage"])
		return
	}
}

func TestBadRequestForUpdateTilesState(t *testing.T) {
	td := &tileDistributorImpl{
		dbh: daotest.TestDBHandler,
		cfg: defaultTestingConfig,
	}
	_, httpServer := setupHTTP(createFakeErrorNotifier(), td, defaultTestingConfig)
	url := fmt.Sprintf("%s/service/v1/tile-state", httpServer.URL)

	tests := []struct {
		requestBody          string
		expectedErrorMessage string
	}{
		{
			`{
			"tileSpecIds": ["1.2.3.4.5", "1.2.6.7.9"]
			}`,
			"Invalid tile state - tile state cannot be empty",
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["1.2.3.4.5", "1"]
			}`,
			"Invalid tile spec ID format: '1' - it must <acqID>.<tileCol>.<tileRow>.<tileSection>.<#_of_prev_acquisitions_for_section>",
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["a.2.3.4.5"]
			}`,
			`Invalid acq ID value a in a.2.3.4.5: strconv.ParseUint: parsing "a": invalid syntax`,
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["1.b.3.4.5"]
			}`,
			`Invalid tile column value b in 1.b.3.4.5: strconv.ParseInt: parsing "b": invalid syntax`,
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["1.2.c.4.5"]
			}`,
			`Invalid tile row value c in 1.2.c.4.5: strconv.ParseInt: parsing "c": invalid syntax`,
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["1.2.3.d.5"]
			}`,
			`Invalid section value d in 1.2.3.d.5: strconv.ParseInt: parsing "d": invalid syntax`,
		},
		{
			`{
			"state": "NEW_STATE",
			"tileSpecIds": ["1.2.3.4.e"]
			}`,
			`Invalid value for number of previous acquisitions e in 1.2.3.4.e: strconv.ParseInt: parsing "e": invalid syntax`,
		},
	}
	for _, testData := range tests {
		bodyReader := strings.NewReader(testData.requestBody)
		request, err := http.NewRequest("PUT", url, bodyReader)
		if err != nil {
			t.Error("Unexpacted error while creating the POST request for", url, err)
		}
		request.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err)
		}
		if res.StatusCode != 400 {
			t.Error("Expected status 400 but instead it got", res.StatusCode)
		}

		var reqResults map[string]interface{}
		bodyBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Error("Error reading update-tiles-state response for ", testData.requestBody, err)
			return
		}
		if err = json.Unmarshal(bodyBytes, &reqResults); err != nil {
			t.Error("Error reading update-tiles-state response for", testData.requestBody, err)
			return
		}
		if reqResults["errormessage"] == nil {
			t.Error("Expected an error message but it didn't get any")
			return
		}
		errorMessage := reqResults["errormessage"].(string)
		if errorMessage != testData.expectedErrorMessage {
			t.Error("Expected error message", testData.expectedErrorMessage, "but got", "'"+errorMessage+"'")
		}
	}
}

func BenchmarkFakeServicePOST5MOnlyCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpPostTile(httpServer.URL, testAcqID, testCamera, testCol, testRow, smallImage)
		}
	})
}

func BenchmarkFakeServicePOST20MOnlyCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpPostTile(httpServer.URL, testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}

func BenchmarkFakeServicePOST5MAnd20MCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			testImageChooser := rand.Intn(3)
			var testImage []byte
			if testImageChooser == 0 {
				testImage = largeImage
			} else {
				testImage = smallImage
			}
			httpPostTile(httpServer.URL, testAcqID, testCamera, testCol, testRow, testImage)
		}
	})
}

func BenchmarkFakeServicePUT5MOnlyCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(httpServer.URL, "PUT", testAcqID, testCamera, testCol, testRow, smallImage)
		}
	})
}

func BenchmarkFakeServicePUT20MOnlyCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			httpSendTile(httpServer.URL, "PUT", testAcqID, testCamera, testCol, testRow, largeImage)
		}
	})
}

func BenchmarkFakeServicePUT5MAnd20MCaptureImageRequests(b *testing.B) {
	_, httpServer := setupHTTP(createFakeErrorNotifier(), nil, defaultTestingConfig)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			testRow := rand.Intn(10000)
			testCol := rand.Intn(10000)
			testCamera := rand.Intn(4)
			testImageChooser := rand.Intn(3)
			var testImage []byte
			if testImageChooser == 0 {
				testImage = largeImage
			} else {
				testImage = smallImage
			}
			httpSendTile(httpServer.URL, "PUT", testAcqID, testCamera, testCol, testRow, testImage)
		}
	})
}
