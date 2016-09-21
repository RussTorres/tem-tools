package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"imagecatcher/protocol"
)

const (
	maxCols = 10000
	maxRows = 10000
)

var sizeRegexp = regexp.MustCompile("(\\d+\\.?\\d*)([a-z,A-Z]?)")

func main() {
	var (
		serviceURL             = flag.String("service-url", "http://localhost:5001", "Service url")
		iterations             = flag.Int("iterations", -1, "Number of iterations")
		totalRunningTimeInMins = flag.Float64("total-running-time", 5, "Total running time")
		runningPeriodInMins    = flag.Float64("running-period-time", 60, "Running period in minutes")
		breakPeriodInMins      = flag.Float64("break-time", 0, "Break period in minutes")
		rateInMillis           = flag.Int("rate", 150, "Rate in milliseconds")
		camera                 = flag.Int("camera", 0, "Camera index")
		imageSizeDesc          = flag.String("image-size", "5.3m", "Image size")
		acqDirURL              = flag.String("acq-dir-url", "", "Acquisition directory url")
		action                 = flag.String("action", "send-tile", "Action: send-tile|send-acq-log|send-rois")
		sendMethod             = flag.String("send-method", "TCP-PUT", "Request method used for sending the tiles {HTTP-PUT|HTTP-POST|TCP-PUT}")
		logfile                = flag.String("logfile", "", "Name of the logfile")
		concurrently           = flag.Bool("concurrently", false, "Flag whether to send the tiles concurrently")
	)
	flag.Parse()
	var imageSize int
	var err error
	if imageSize, err = parseImageSizeDesc(*imageSizeDesc); err != nil {
		log.Panicf("Invalid image size %s\n", err)
	}
	initializeLog(*logfile)
	if *acqDirURL == "" {
		fmt.Print("Acquisition dir is required")
		flag.PrintDefaults()
		os.Exit(1)
	}
	acqID := extractAcqID(*acqDirURL)
	switch *action {
	case "send-acq-log":
		sendAcqLog(*serviceURL, *acqDirURL)
	case "send-rois":
		sendRois(*serviceURL, *acqDirURL, acqID)
	case "send-tile":
		sendTiles(*serviceURL, acqID, *camera, imageSize, *iterations, *rateInMillis, int(*totalRunningTimeInMins*60), int(*runningPeriodInMins*60), int(*breakPeriodInMins*60), *sendMethod, *concurrently)
	}
}

func parseImageSizeDesc(sizeDesc string) (size int, err error) {
	if sizeRegexp.MatchString(sizeDesc) {
		matches := sizeRegexp.FindStringSubmatch(sizeDesc)
		var sizeValue float64
		if sizeValue, err = strconv.ParseFloat(matches[1], 64); err != nil {
			log.Printf("Error parsing the image size %s: %v", sizeDesc, err)
			return 0, nil
		}
		multiplier := matches[2]
		switch true {
		case multiplier == "k" || multiplier == "K":
			return int(sizeValue * (1 << 10)), nil
		case multiplier == "m" || multiplier == "M":
			return int(sizeValue * (1 << 20)), nil
		}
		return int(sizeValue), nil
	}
	return 0, fmt.Errorf("Invalid size descriptor %s", sizeDesc)
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
		log.Panicf("Error extracting the acqID from %s: %s", acqDirURL, err)
	}
	return
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
	fileName := acqDirName + fileNameSuffix
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

func sendRois(serviceURL, acqDirURL string, acqID uint64) {
	var err error

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

type tileSender struct {
	wg *sync.WaitGroup
}

type sendTileRequest struct {
	base         protocol.CaptureImageRequest
	serviceURL   string
	tileFileName string
	timeLimit    time.Duration
}

func sendTiles(serviceURL string, acqID uint64, camera, size, iterations, rateInMillis, totalRunningTimeInSecs, runningPeriodInSecs, breakPeriodInSecs int, method string, concurrently bool) {
	var col, row int
	var rate, totalRunningTime, runningPeriod, breakPeriod time.Duration
	var err error

	quit := make(chan struct{}, 1)
	ts := tileSender{
		wg: &sync.WaitGroup{},
	}

	if rate, err = time.ParseDuration(fmt.Sprintf("%dms", rateInMillis)); err != nil {
		log.Panicf("Invalid rate")
	}
	if totalRunningTime, err = time.ParseDuration(fmt.Sprintf("%ds", totalRunningTimeInSecs)); err != nil {
		log.Panicf("Invalid total running time")
	}
	if runningPeriod, err = time.ParseDuration(fmt.Sprintf("%ds", runningPeriodInSecs)); err != nil {
		log.Panicf("Invalid running period")
	}
	if breakPeriod, err = time.ParseDuration(fmt.Sprintf("%ds", breakPeriodInSecs)); err != nil {
		log.Panicf("Invalid break period")
	}
	iteration := 0
	imageBuffer, checksum := createTestImage(size)
	var sendTileFunc func(*sendTileRequest)
	switch method {
	case "HTTP-PUT":
		sendTileFunc = func(r *sendTileRequest) {
			tr := &http.Transport{
				ResponseHeaderTimeout: 120 * time.Second,
				DisableCompression:    true,
				MaxIdleConnsPerHost:   0,
				DisableKeepAlives:     true,
			}
			ts.httpPutTile(r, tr)
		}
	case "HTTP-POST":
		sendTileFunc = func(r *sendTileRequest) {
			ts.httpPostTile(r, http.Client{})
		}
	case "TCP-PUT":
		tcpconn, err := net.Dial("tcp", serviceURL)
		if err != nil {
			log.Panicf("Error opening the tcp connection to %s: %v", serviceURL, err)
		}
		sendTileFunc = func(r *sendTileRequest) {
			var conn net.Conn
			if concurrently {
				conn, err = net.Dial("tcp", serviceURL)
				if err != nil {
					log.Printf("Error opening the tcp connection to %s: %v", serviceURL, err)
					return
				}
				defer conn.Close()
			} else {
				conn = tcpconn
			}
			ts.tcpPutTile(r, conn)
		}
	default:
		log.Panicf("Invalid send tile method - supported methods are : HTTP-PUT | HTTP-POST | TCP-PUT")
	}

	time.AfterFunc(totalRunningTime, func() {
		log.Printf("Quit")
		var q struct{}
		quit <- q
	})
	startPeriod := time.Now()
	for row = 0; ; row++ {
		if row >= maxRows {
			row = 0
		}
		for col = 0; ; col++ {
			if col >= maxCols {
				col = 0
			}
			if iterations > 0 && iteration >= iterations {
				return
			}
			tileRequest := &sendTileRequest{
				base: protocol.CaptureImageRequest{
					AcqID:    acqID,
					Camera:   int32(camera),
					Frame:    -1,
					Col:      int32(col),
					Row:      int32(row),
					Image:    imageBuffer,
					Checksum: checksum,
				},
				serviceURL: serviceURL,
				timeLimit:  rate,
			}
			select {
			case <-time.After(rate):
				if concurrently {
					go ts.sendTile(sendTileFunc, tileRequest)
				} else {
					ts.sendTile(sendTileFunc, tileRequest)
				}
			case <-quit:
				ts.wg.Wait()
				return
			}
			iteration++
			if time.Now().After(startPeriod.Add(runningPeriod)) {
				log.Printf("Taking a break for %v", breakPeriod)
				time.Sleep(breakPeriod)
				startPeriod = time.Now()
			}

		}
	}
}

func (ts tileSender) sendTile(sendTileFunc func(*sendTileRequest), req *sendTileRequest) {
	ts.wg.Add(1)
	defer ts.wg.Done()
	sendTileFunc(req)
}

func (ts tileSender) httpPostTile(req *sendTileRequest, httpConn http.Client) {
	startTime := time.Now()
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d", req.serviceURL, req.base.AcqID)
	tileFileName := fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", req.base.Col, req.base.Col, req.base.Row, req.base.Camera)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.base.AcqID, tileFileName, len(req.base.Image), storeTileEndpoint)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("tile-file", tileFileName)
	if err != nil {
		log.Printf("Error creating the request for %s: %s", tileFileName, err)
		return
	}
	_, err = io.Copy(part, bytes.NewReader(req.base.Image))

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
		log.Printf("Error sending %d:%s request: %s", req.base.AcqID, tileFileName, err)
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
	log.Printf("Sent %d:%s (%d) in %v, time left %d ms - status %d, resp %v", req.base.AcqID, tileFileName,
		len(req.base.Image), actualExecTime, (req.timeLimit.Nanoseconds()-actualExecTime.Nanoseconds())/1000000,
		res.StatusCode, tileInfo)
}

func (ts tileSender) httpPutTile(req *sendTileRequest, tr *http.Transport) {
	startTime := time.Now()
	tileFileName := fmt.Sprintf("col%04d/col%04d_row%04d_cam%d.tif", req.base.Col, req.base.Col, req.base.Row, req.base.Camera)
	storeTileEndpoint := fmt.Sprintf("%s/service/v1/capture-image-content/%d?tile-filename=%s",
		req.serviceURL, req.base.AcqID, tileFileName)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.base.AcqID, tileFileName, len(req.base.Image), storeTileEndpoint)

	request, err := http.NewRequest("PUT", storeTileEndpoint, bytes.NewReader(req.base.Image))
	if err != nil {
		log.Printf("Error creating the PUT request for %s: %s", tileFileName, err)
		return
	}
	res, err := tr.RoundTrip(request)
	if err != nil {
		log.Printf("Error sending %d:%s (%d bytes) to %s: %v", req.base.AcqID, tileFileName, len(req.base.Image), storeTileEndpoint, err)
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
	log.Printf("Sent %d:%s (%d) in %v, time left %d ms - status %d, resp %v, headers: %v", req.base.AcqID, tileFileName,
		len(req.base.Image), actualExecTime, (req.timeLimit.Nanoseconds()-actualExecTime.Nanoseconds())/1000000,
		res.StatusCode, tileInfo, res.Header)
}

func (ts tileSender) tcpPutTile(req *sendTileRequest, tcpconn net.Conn) {
	startTime := time.Now()
	req.tileFileName = fmt.Sprintf("col%04d_row%04d_cam%d.tif", req.base.Col, req.base.Row, req.base.Camera)
	log.Printf("Sending %d:%s (%d bytes) to %s", req.base.AcqID, req.tileFileName, len(req.base.Image), req.serviceURL)

	tileReqBuf := protocol.MarshalTileRequest(&req.base)
	if err := protocol.WriteUint32(tcpconn, uint32(len(tileReqBuf))); err != nil {
		log.Printf("Error writing the total buffer length for %d:%s (%d bytes) to %s: %v",
			req.base.AcqID, req.tileFileName, len(req.base.Image), req.serviceURL, err)
	}
	if _, err := tcpconn.Write(tileReqBuf); err != nil {
		log.Printf("Error writing the buffer %d:%s (%d bytes) to %s: %v", req.base.AcqID, req.tileFileName, len(req.base.Image), req.serviceURL, err)
	}

	responseBufferLength, err := protocol.ReadUint32(tcpconn)
	if err != nil {
		log.Printf("Error reading the response buffer length %d:%s (%d bytes) to %s: %v",
			req.base.AcqID, req.tileFileName, len(req.base.Image), req.serviceURL, err)
	}
	// allocate a little bit more than the request buffer length to avoid reallocations
	responseBuffer := bytes.NewBuffer(make([]byte, 0, responseBufferLength+bytes.MinRead+1))
	n, err := responseBuffer.ReadFrom(io.LimitReader(tcpconn, int64(responseBufferLength)))
	if err != nil {
		log.Printf("Error reading the response buffer %d:%s (%d bytes) to %s: %v",
			req.base.AcqID, req.tileFileName, len(req.base.Image), req.serviceURL, err)
	}
	if uint32(n) < responseBufferLength {
		log.Printf("Expected to read %d bytes but instead it only read %d. This may result into a 'Unmarshalling error'!", responseBufferLength, n)
	}

	imageTileResponse := protocol.UnmarshalTileResponse(responseBuffer.Bytes())
	actualExecTime := time.Since(startTime)
	log.Printf("Sent %d:%s (%d) in %v, request status - %d, tileId - %d, content queue status - %s, tile queue status - %s, system status - %s, time left %d ms",
		imageTileResponse.AcqID, req.tileFileName,
		len(req.base.Image), actualExecTime,
		imageTileResponse.Status,
		imageTileResponse.TileID,
		queueStatusToString(imageTileResponse.ContentQueueStatus),
		queueStatusToString(imageTileResponse.TileQueueStatus),
		queueStatusToString(imageTileResponse.SystemStatus),
		(req.timeLimit.Nanoseconds()-actualExecTime.Nanoseconds())/1000000)
}

func createTestImage(size int) (content, md5checksum []byte) {
	content = make([]byte, size)
	for i := range content {
		content[i] = byte(rand.Intn(256))
	}
	checksum := md5.Sum(content)
	md5checksum = checksum[0:]
	return
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
