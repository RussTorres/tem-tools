package main

import (
	"bytes"
	"crypto/md5"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"imagecatcher/protocol"
)

const (
	maxCameras = 4
	maxCols    = 10000
	maxRows    = 10000
)

var sizeRegexp = regexp.MustCompile("(\\d+\\.?\\d*)([a-z,A-Z]?)")

type tileInfo struct {
	camera, col, row int
}

// String representation of a tileInfo
func (ti tileInfo) String() string {
	return fmt.Sprintf("Cam: %d, tile: (%d, %d)", ti.camera, ti.col, ti.row)
}

func main() {
	var (
		serviceURL     = flag.String("service-url", "http://localhost:5001", "Service url")
		acqServiceURL  = flag.String("acq-service-url", "", "Service url")
		tileServiceURL = flag.String("tile-service-url", "", "Service url")
		acqDirURL      = flag.String("acq-dir-url", "", "Acquisition directory url")
		camera         = flag.Int("camera", 0, "Camera index")
		col            = flag.Int("col", 0, "Tile col")
		row            = flag.Int("row", 0, "Tile row")
		rateInMillis   = flag.Int("rate", 0, "Rate in milliseconds")
		action         = flag.String("action", "send-tile", "Action: send-acq|send-tile|send-acq-log|send-rois")
		sendMethod     = flag.String("send-method", "TCP-PUT", "Request method used for sending the tiles {HTTP-PUT|HTTP-POST|TCP-PUT}")
		logfile        = flag.String("logfile", "", "Name of the logfile")
	)
	flag.Parse()
	initializeLog(*logfile)
	if *acqDirURL == "" {
		fmt.Print("Acquisition dir is required")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *acqServiceURL == "" {
		*acqServiceURL = *serviceURL
	}
	if *tileServiceURL == "" {
		*tileServiceURL = *serviceURL
	}
	switch *action {
	case "send-acq":
		sendAcq(*acqServiceURL, *tileServiceURL, *acqDirURL, *camera, *rateInMillis, *sendMethod)
	case "send-acq-log":
		sendAcqLog(*acqServiceURL, *acqDirURL)
	case "send-rois":
		sendRois(*acqServiceURL, *acqDirURL)
	case "send-tile":
		sendImageTile(*tileServiceURL, *acqDirURL, *camera, *col, *row, *sendMethod)
	}
}

func initializeLog(logfile string) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	if logfile == "" {
		return
	}
	logDir := path.Dir(logfile)
	if logDir != "." && logDir != ".." {
		err := os.MkdirAll(logDir, os.ModePerm)
		if err != nil {
			log.Print(err)
			return
		}
	}
	lw, err := os.Create(logfile)
	if err != nil {
		log.Print(err)
		return
	}
	log.SetOutput(lw)
}

func extractAcqID(acqDirURL string) (acqID uint64) {
	var err error
	acqDirName := filepath.Base(acqDirURL)
	if acqID, err = strconv.ParseUint(strings.Split(acqDirName, "_")[0], 10, 64); err != nil {
		log.Panicf("Error extracting the acqId from %s: %s", acqDirURL, err)
	}
	return
}

func sendAcq(acqServiceURL, tileServiceURL, acqDirURL string, camera int, rateInMillis int, tileSendMethod string) {
	sendAcqLog(acqServiceURL, acqDirURL)
	sendRois(acqServiceURL, acqDirURL)

	timingFileName := getAcqFileName(acqDirURL, "_DAQ timing.csv")

	rate := time.Duration(rateInMillis) * time.Millisecond

	stopChan := make(chan os.Signal)
	var quit bool
	quitChan := make(chan bool, 1)
	go func() {
		<-stopChan
		quit = true
		quitChan <- quit
	}()
	signal.Notify(stopChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGKILL)

	tch, err := getAcqTiles(acqDirURL, timingFileName, camera, rate, quitChan)
	if err != nil {
		log.Printf("%v", err)
		return
	}

	var step, nWorkers int
	if camera == -1 {
		nWorkers = maxCameras
	} else {
		nWorkers = 1
	}
	tileSendersCh := make(chan tileInfo, nWorkers)
	wp := imageSendWorkerPool{
		tileServiceURL: tileServiceURL,
		acqDirURL:      acqDirURL,
		tileSendMethod: tileSendMethod,
		workersTileCh:  tileSendersCh,
	}
	wp.startWorkers(nWorkers)
	for cameraCh := range tch {
		for ti := range cameraCh {
			log.Printf("Step %d: Send %v", step, ti)
			tileSendersCh <- ti
		}
		step++
	}
	close(tileSendersCh)
	if !quit {
		endAcq(acqServiceURL, acqDirURL)
	}
}

type imageSendWorkerPool struct {
	tileServiceURL string
	acqDirURL      string
	tileSendMethod string
	workersTileCh  chan tileInfo
}

func (iswp imageSendWorkerPool) startWorkers(nWorkers int) {
	for i := 0; i < nWorkers; i++ {
		w := imageSendWorker{
			tileServiceURL: iswp.tileServiceURL,
			acqDirURL:      iswp.acqDirURL,
			tileSendMethod: iswp.tileSendMethod,
		}
		go w.run(iswp.workersTileCh)
	}
}

type imageSendWorker struct {
	tileServiceURL string
	acqDirURL      string
	tileSendMethod string
}

func (isw imageSendWorker) run(tilesCh chan tileInfo) {
	for ti := range tilesCh {
		sendImageTile(isw.tileServiceURL, isw.acqDirURL, ti.camera, ti.col, ti.row, isw.tileSendMethod)
	}
}

func getAcqTiles(acqDirURL, timingFileName string, camera int, rate time.Duration, quit <-chan bool) (<-chan chan tileInfo, error) {

	tf, err := os.Open(timingFileName)
	if err != nil {
		return nil, fmt.Errorf("Error opening the timing file %s: %v", timingFileName, err)
	}
	defer tf.Close()

	csvReader := csv.NewReader(tf)
	csvReader.LazyQuotes = true
	csvRecords, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("Error while parsing the timing file %s: %v", timingFileName, err)
	}

	tiCh := make(chan chan tileInfo)

	go func() {
		headers := make(map[string]int)
		for i, h := range csvRecords[0] {
			headers[h] = i
		}
		var step int
		for _, r := range csvRecords[1:] {
			var camerasCh chan tileInfo
			if camera < 0 {
				camerasCh = make(chan tileInfo, maxCameras)
				for cameraIndex := 0; cameraIndex < maxCameras; cameraIndex++ {
					col, row := getTileCoordForCamera(acqDirURL, r, headers, cameraIndex)
					if col != -1 && row != -1 {
						ti := tileInfo{
							camera: cameraIndex,
							col:    col,
							row:    row,
						}
						log.Printf("Step %d: Prepare %v", step, ti)
						camerasCh <- ti
					}
				}
			} else {
				camerasCh = make(chan tileInfo, 1)
				col, row := getTileCoordForCamera(acqDirURL, r, headers, camera)
				if col != -1 && row != -1 {
					ti := tileInfo{
						camera: camera,
						col:    col,
						row:    row,
					}
					log.Printf("Step %d: Prepare %v", step, ti)
					camerasCh <- ti
				}
			}
			close(camerasCh)
			tiCh <- camerasCh
			select {
			case <-time.After(rate):
				break
			case q := <-quit:
				if q {
					close(tiCh)
					return
				}
			}
			step++
		}
		close(tiCh)
	}()
	return tiCh, nil
}

func getTileCoordForCamera(acqDirURL string, values []string, headings map[string]int, camera int) (int, int) {
	colField := strings.TrimSpace(values[headings[fmt.Sprintf("Cam %d Col", camera)]])
	rowField := strings.TrimSpace(values[headings[fmt.Sprintf("Cam %d Row", camera)]])
	if colField == "" || rowField == "" {
		return -1, -1
	}
	col, err := strconv.Atoi(colField)
	if err != nil {
		log.Printf("WARNING: Invalid col: %s in %v", colField, values)
		return -1, -1
	}
	row, err := strconv.Atoi(rowField)
	if err != nil {
		log.Printf("WARNING: Invalid row: %s in %v", rowField, values)
		return -1, -1
	}
	return col, row
}

func sendAcqLog(serviceURL, acqDirURL string) {
	acqLogFilename := getAcqFileName(acqDirURL, "_logfile.ini")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := addFilePart(writer, "acq-inilog", acqLogFilename)
	if err != nil {
		log.Printf("Error attaching the acq log to the request %s: %s", acqLogFilename, err)
		return
	}
	err = writer.Close()
	if err != nil {
		log.Printf("Error closing the multipart writer for %s: %s", acqLogFilename, err)
		return
	}
	acqEndpoint := fmt.Sprintf("%s/service/v1/start-acquisition", serviceURL)
	request, err := http.NewRequest("POST", acqEndpoint, body)
	if err != nil {
		log.Printf("Error creating the POST request for %s: %s", acqLogFilename, err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("Error sending %s request: %s", acqLogFilename, err)
		return
	}
	defer res.Body.Close()
	var acqInfo map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading the response from %s: %s", acqLogFilename, err)
		return
	}
	if err = json.Unmarshal(bodyBytes, &acqInfo); err != nil {
		log.Printf("Error decoding response for %s: %s", acqLogFilename, err)
		return
	}
	acqID := uint64(acqInfo["uid"].(float64))
	log.Printf("Sent %s - status %d, resp %v: %d", acqLogFilename, res.StatusCode, acqInfo, acqID)
}

func getAcqFileName(acqDirURL, fileNameSuffix string) string {
	acqDir := acqDirURL
	if strings.HasPrefix(acqDir, "file://") {
		acqDir = strings.TrimPrefix(acqDir, "file://")
	}
	acqDirName := filepath.Base(acqDirURL)
	fileNamePattern := acqDirName + "*" + fileNameSuffix
	fullNamePattern := filepath.Join(acqDir, fileNamePattern)
	matches, err := filepath.Glob(fullNamePattern)
	if err != nil {
		log.Printf("Error looking up %s: %v", fullNamePattern, err)
		return ""
	}
	if len(matches) == 0 {
		log.Printf("No match found for %s", fullNamePattern)
		return ""
	}
	if len(matches) > 1 {
		log.Printf("Too many matches found for %s", fullNamePattern)
		return ""
	}
	return matches[0]
}

func getAcqFile(acqDirURL, fileName string) string {
	acqDir := acqDirURL
	if strings.HasPrefix(acqDir, "file://") {
		acqDir = strings.TrimPrefix(acqDir, "file://")
	}
	return filepath.Join(acqDir, fileName)
}

func addFilePart(formWriter *multipart.Writer, formFieldName, filename string) error {
	part, err := formWriter.CreateFormFile(formFieldName, filename)
	if err != nil {
		log.Printf("Error creating the request for %s: %s", filename, err)
		return err
	}
	f, err := os.Open(filename)
	if err != nil {
		log.Printf("Error opening the acquisition file %s: %s", filename, err)
		return err
	}
	_, err = io.Copy(part, f)
	if err != nil {
		log.Printf("Error copying the content of the %s into the request: %s", filename, err)
	}
	f.Close()
	return nil // even if close fail ignore it if everything went well so far
}

func sendRois(serviceURL, acqDirURL string) {
	var err error

	acqID := extractAcqID(acqDirURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	roiTilesFilename := getAcqFileName(acqDirURL, "_ROI_tiles.csv")
	if err = addFilePart(writer, "roi-tiles", roiTilesFilename); err != nil {
		log.Printf("Error attaching ROI Tiles file to the request %s: %s", roiTilesFilename, err)
		return
	}

	roiSpecFilename := getAcqFileName(acqDirURL, "_ROI_spec.ini")
	if err = addFilePart(writer, "roi-spec", roiSpecFilename); err != nil {
		log.Printf("Error attaching ROI Spec file to the request %s: %s", roiSpecFilename, err)
		return
	}

	if err = writer.Close(); err != nil {
		log.Printf("Error closing the multipart ROI writer for %s", err)
		return
	}

	roisEndpoint := fmt.Sprintf("%s/service/v1/create-rois/%d", serviceURL, acqID)
	request, err := http.NewRequest("POST", roisEndpoint, body)
	if err != nil {
		log.Printf("Error creating the POST request %s for %s: %s", roisEndpoint, acqDirURL, err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("Error sending request %s for %s: %s", roisEndpoint, acqDirURL, err)
		return
	}
	defer res.Body.Close()
	log.Printf("Sent %s to %s - status %d", acqDirURL, roisEndpoint, res.StatusCode)
}

func endAcq(serviceURL, acqDirURL string) {
	var err error

	acqID := extractAcqID(acqDirURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	mosaicFilename := getAcqFileName(acqDirURL, "_mosaic.png")
	if err = addFilePart(writer, "ancillary-file", mosaicFilename); err != nil {
		log.Printf("Error attaching mosaic file to the request %s: %s", mosaicFilename, err)
		return
	}

	if err = writer.Close(); err != nil {
		log.Printf("Error closing the multipart writer for %s", err)
		return
	}

	endpoint := fmt.Sprintf("%s/service/v1/end-acquisition/%d", serviceURL, acqID)
	request, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		log.Printf("Error creating the POST request %s for %s: %s", endpoint, acqDirURL, err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("Error sending request %s for %s: %s", endpoint, acqDirURL, err)
		return
	}
	defer res.Body.Close()
	log.Printf("Sent %s to %s - status %d", acqDirURL, endpoint, res.StatusCode)
}

func sendImageTile(serviceURL, acqDirURL string, camera, col, row int, method string) {
	var err error

	acqID := extractAcqID(acqDirURL)
	acqTileName := getAcqFile(acqDirURL, fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", col, col, row, camera))
	imageBuffer, err := ioutil.ReadFile(acqTileName)
	if err != nil {
		log.Fatal(err)
	}
	checksum := md5.Sum(imageBuffer)
	var sendTileFunc func(*protocol.CaptureImageRequest)
	switch method {
	case "HTTP-PUT":
		sendTileFunc = func(r *protocol.CaptureImageRequest) {
			tr := &http.Transport{
				ResponseHeaderTimeout: 120 * time.Second,
				DisableCompression:    true,
				MaxIdleConnsPerHost:   0,
				DisableKeepAlives:     true,
			}
			httpPutTile(serviceURL, r, tr)
		}
	case "HTTP-POST":
		sendTileFunc = func(r *protocol.CaptureImageRequest) {
			httpPostTile(serviceURL, r, http.Client{})
		}
	case "TCP-PUT":
		sendTileFunc = func(r *protocol.CaptureImageRequest) {
			var conn net.Conn
			conn, err = net.Dial("tcp", serviceURL)
			if err != nil {
				log.Printf("Error opening the tcp connection to %s: %v", serviceURL, err)
				return
			}
			defer conn.Close()
			tcpPutTile(serviceURL, r, conn)
		}
	default:
		log.Panicf("Invalid send tile method - supported methods are : HTTP-PUT | HTTP-POST | TCP-PUT")
	}

	tileRequest := &protocol.CaptureImageRequest{
		AcqID:    acqID,
		Camera:   int32(camera),
		Frame:    -1,
		Col:      int32(col),
		Row:      int32(row),
		Image:    imageBuffer,
		Checksum: checksum[0:],
	}
	sendTileFunc(tileRequest)
}

func httpPostTile(serviceURL string, req *protocol.CaptureImageRequest, httpConn http.Client) {
	startTime := time.Now()
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d", serviceURL, req.AcqID)
	tileFileName := fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", req.Col, req.Col, req.Row, req.Camera)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.AcqID, tileFileName, len(req.Image), storeTileEndpoint)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("tile-file", tileFileName)
	if err != nil {
		log.Printf("Error creating the request for %s: %s", tileFileName, err)
		return
	}
	_, err = io.Copy(part, bytes.NewReader(req.Image))

	err = writer.WriteField("tile-filename", tileFileName)
	if err != nil {
		log.Printf("Error writing the tile file name %s: %s", tileFileName, err)
		return
	}
	if err = writer.Close(); err != nil {
		log.Printf("Error closing the multipart writer for %s: %s", tileFileName, err)
		return
	}

	request, err := http.NewRequest("POST", storeTileEndpoint, body)
	if err != nil {
		log.Printf("Error creating the POST request for %s: %s", tileFileName, err)
		return
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := httpConn.Do(request)
	if err != nil {
		log.Printf("Error sending %d:%s request: %s", req.AcqID, tileFileName, err)
		return
	}
	defer res.Body.Close()
	var tileInfo map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	actualExecTime := time.Since(startTime)
	if err != nil {
		log.Printf("Error reading body response for %s: %s", tileFileName, err)
	} else {
		err = json.Unmarshal(bodyBytes, &tileInfo)
		if err != nil {
			log.Printf("Error decoding response for %s: %s", tileFileName, err)
		}
	}
	log.Printf("Sent %d:%s (%d) in %v - status %d, resp %v", req.AcqID, tileFileName,
		len(req.Image), actualExecTime, res.StatusCode, tileInfo)
}

func httpPutTile(serviceURL string, req *protocol.CaptureImageRequest, tr *http.Transport) {
	startTime := time.Now()
	tileFileName := fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", req.Col, req.Col, req.Row, req.Camera)
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d?tile-filename=%s",
		serviceURL, req.AcqID, tileFileName)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.AcqID, tileFileName, len(req.Image), storeTileEndpoint)

	request, err := http.NewRequest("PUT", storeTileEndpoint, bytes.NewReader(req.Image))
	if err != nil {
		log.Printf("Error creating the PUT request for %s: %s", tileFileName, err)
		return
	}
	res, err := tr.RoundTrip(request)
	if err != nil {
		log.Printf("Error sending %d:%s (%d bytes) to %s: %v", req.AcqID, tileFileName, len(req.Image), storeTileEndpoint, err)
		return
	}
	defer res.Body.Close()
	var tileInfo map[string]interface{}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	actualExecTime := time.Since(startTime)
	if err != nil {
		log.Printf("Error reading body response for %s: %s", tileFileName, err)
	} else {
		err = json.Unmarshal(bodyBytes, &tileInfo)
		if err != nil {
			log.Printf("Error decoding response for %s: %s", tileFileName, err)
		}
	}
	log.Printf("Sent %d:%s (%d) in %v - status %d, resp %v, headers: %v", req.AcqID, tileFileName,
		len(req.Image), actualExecTime, res.StatusCode, tileInfo, res.Header)
}

func tcpPutTile(serviceURL string, req *protocol.CaptureImageRequest, tcpconn net.Conn) {
	startTime := time.Now()
	tileFileName := fmt.Sprintf("col%04d_row%04d_cam%d.tif", req.Col, req.Row, req.Camera)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.AcqID, tileFileName, len(req.Image), serviceURL)

	tileReqBuf := protocol.MarshalTileRequest(req)
	if err := protocol.WriteUint32(tcpconn, uint32(len(tileReqBuf))); err != nil {
		log.Printf("Error writing the total buffer length for %d:%s (%d bytes) to %s: %v",
			req.AcqID, tileFileName, len(req.Image), serviceURL, err)
	}
	if _, err := tcpconn.Write(tileReqBuf); err != nil {
		log.Printf("Error writing the buffer %d:%s (%d bytes) to %s: %v", req.AcqID, tileFileName, len(req.Image), serviceURL, err)
	}

	responseBufferLength, err := protocol.ReadUint32(tcpconn)
	if err != nil {
		log.Printf("Error reading the response buffer length %d:%s (%d bytes) to %s: %v",
			req.AcqID, tileFileName, len(req.Image), serviceURL, err)
	}
	// allocate a little bit more than the request buffer length to avoid reallocations
	responseBuffer := bytes.NewBuffer(make([]byte, 0, responseBufferLength+1024))
	n, err := responseBuffer.ReadFrom(io.LimitReader(tcpconn, int64(responseBufferLength)))
	if err != nil {
		log.Printf("Error reading the response buffer %d:%s (%d bytes) to %s: %v",
			req.AcqID, tileFileName, len(req.Image), serviceURL, err)
	}
	if uint32(n) < responseBufferLength {
		log.Printf("Expected to read %d bytes but instead it only read %d. This may result into a 'Unmarshalling error'!", responseBufferLength, n)
	}

	imageTileResponse := protocol.UnmarshalTileResponse(responseBuffer.Bytes())
	actualExecTime := time.Since(startTime)
	log.Printf("Sent %d:%s (%d) in %v, request status - %d, tileId - %d, content queue status - %s, tile queue status - %s, system status - %s",
		imageTileResponse.AcqID, tileFileName,
		len(req.Image), actualExecTime,
		imageTileResponse.Status,
		imageTileResponse.TileID,
		queueStatusToString(imageTileResponse.ContentQueueStatus),
		queueStatusToString(imageTileResponse.TileQueueStatus),
		queueStatusToString(imageTileResponse.SystemStatus),
	)
}

func queueStatusToString(qs int16) string {
	switch qs {
	case 0:
		return "GREEN"
	case 1:
		return "YELLOW"
	case 2:
		return "RED"
	default:
		return "INVALID"
	}
}
