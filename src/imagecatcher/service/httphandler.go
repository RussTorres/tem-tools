package service

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"imagecatcher/logger"
	"imagecatcher/models"
	"imagecatcher/utils"
)

// httpServerHandler handler for HTTP requests
type httpServerHandler struct {
	Listener        net.Listener
	httpImpl        *http.Server
	imageCatcher    ImageCatcherService
	trh             *TileRequestHandler
	tileDistributor TileDistributor
	configurator    Configurator
	messageNotifier MessageNotifier
}

var errWrongExtractMethod = errors.New("Extractor cannot be used for this request")
var timeZone, utcTimeZone *time.Location

func init() {
	var err error
	if timeZone, err = time.LoadLocation("Local"); err != nil {
		logger.Errorf("Error loading Local time zone")
	}
	if utcTimeZone, err = time.LoadLocation("UTC"); err != nil {
		logger.Errorf("Error loading UTC time zone")
	}
}

// Serve implementation of a ServerHandler method
func (h *httpServerHandler) Serve() error {
	if h.Listener == nil {
		return h.httpImpl.ListenAndServe()
	}
	return h.httpImpl.Serve(h.Listener)
}

// NewHTTPServerHandler creates an instance of an HTTPHandler
func NewHTTPServerHandler(l net.Listener,
	s ImageCatcherService,
	trh *TileRequestHandler,
	td TileDistributor,
	configurator Configurator,
	messageNotifier MessageNotifier,
	keepAlives bool) ServerHandler {
	router := httprouter.New()

	httpImpl := &http.Server{Handler: router}
	httpImpl.SetKeepAlivesEnabled(keepAlives)

	h := &httpServerHandler{
		Listener:        l,
		httpImpl:        httpImpl,
		imageCatcher:    s,
		trh:             trh,
		tileDistributor: td,
		configurator:    configurator,
		messageNotifier: messageNotifier,
	}

	router.GET("/service/v1/ping", h.ping)
	// Acquisition Capture Endpoints
	router.POST("/service/v1/start-acquisition", h.startAcquisition)
	router.POST("/service/v1/end-acquisition/:acqid", h.endAcquisition)
	router.POST("/service/v1/create-rois/:acqid", h.createROIs)
	router.POST("/service/v1/capture-image-content/:acqid", h.captureImage)
	router.POST("/service/v1/store-ancillary-files/:acqid", h.storeAncillaryFiles)
	router.PUT("/service/v1/capture-image-content/:acqid", h.captureImage)
	// Acquisition Data Endpoints
	router.GET("/service/v1/projects", h.getAcquisitionProjects)
	router.GET("/service/v1/acquisitions", h.getAcquisitions)
	router.GET("/service/v1/acquisition/:acqid/tiles", h.getAcqTiles)
	router.GET("/service/v1/acquisition/:acqid/tile/:col/:row", h.getTileByTileCoord)
	router.GET("/service/v1/acquisition/:acqid/tile/:col/:row/content", h.getTileContentByTileCoord)
	router.GET("/service/v1/verify-tile/:acqid/tile/:tileid", h.verifyTile)
	// Tile Data Endpoints
	router.GET("/service/v1/tiles", h.getAllTiles)
	router.GET("/service/v1/tile/:tileid", h.getTileByTileID)
	router.GET("/service/v1/tile/:tileid/content", h.getTileContentByTileID)
	// Calibration Endpoints
	router.POST("/service/v1/calibrations", h.createCalibrations)
	router.GET("/service/v1/calibrations", h.getCalibrations)
	router.GET("/service/v1/calibration/name/:calibration_name", h.getCalibrationsByName)
	// "Renderer" Endpoints
	router.POST("/service/v1/next-tile", h.serveNextTiles)
	router.PUT("/service/v1/tile-state", h.updateTilesState)

	return h
}

// ping the system to see if it's up
func (h *httpServerHandler) ping(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	err := h.imageCatcher.Ping()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeQueueStatus(w)
	w.WriteHeader(http.StatusOK)
}

func (h *httpServerHandler) writeQueueStatus(w http.ResponseWriter) {
	systemStatus, tileQueueStatus, contentQueueStatus := h.trh.getProcessingQueuesStatus()
	w.Header().Set("System Status", systemStatus.String())
	w.Header().Set("Tile Queue Status", tileQueueStatus.String())
	w.Header().Set("Content Queue Status", contentQueueStatus.String())
}

func (h *httpServerHandler) startAcquisition(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	acqLog := &FileParams{}
	err := extractAcqLogFromMPRequest(r, acqLog)
	if err != nil {
		acqLogErr := fmt.Errorf("Error extracting the acquisition log content: %v", err)
		logger.Error(acqLogErr)
		writeError(w, acqLogErr, http.StatusBadRequest)
		return
	}
	if len(acqLog.Content) == 0 {
		err = fmt.Errorf("No acquisition log found in start-acquisition request")
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	acq, err := h.imageCatcher.CreateAcquisition(acqLog)
	if err != nil {
		err = fmt.Errorf("Error encountered while processing the acquisition log in start-acquisition request: %v", err)
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(formatAcquisitionResponse(*acq))
}

func extractAcqLogFromMPRequest(r *http.Request, f *FileParams) error {
	formReader, err := r.MultipartReader()
	if err != nil {
		return err
	}
	for {
		p, err := formReader.NextPart()
		if err == io.EOF {
			return nil
		}
		ok, err := extractFileField(p, "acq-inilog", f)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
}

func writeError(w http.ResponseWriter, errmsgs interface{}, status int) {
	w.WriteHeader(status)
	var errresp []byte
	var merr error
	switch v := errmsgs.(type) {
	case error:
		errresp, merr = json.Marshal(map[string]string{
			"errormessage": v.Error(),
		})
	default:
		logger.Errorf("Internal Error - Unsupported interface passed in for 'errormessage'")
	}
	if merr != nil {
		logger.Errorf("Error marshalling the error response: %v", merr)
	}
	w.Write(errresp)
}

func formatAcquisitionResponse(acq models.Acquisition) map[string]interface{} {
	r := make(map[string]interface{})
	r["uid"] = acq.AcqUID
	return r
}

func (h *httpServerHandler) endAcquisition(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var acqID uint64
	var err error
	w.Header().Set("Content-Type", "application/json")

	if acqID, err = parseRequiredParamValueAsUint64("acqid", params.ByName("acqid")); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	acqMosaic, err := h.imageCatcher.GetMosaic(acqID)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	h.processAncillaryFiles(w, r, acqMosaic)
	if err = h.imageCatcher.EndAcquisition(acqID); err != nil {
		logger.Error("Error finishing the acquisition: ", acqID, err)
		writeError(w, err, http.StatusInternalServerError)
	}
	// perform a quick verification to see if the current acquisition still has tiles in the create state
	var acqFilter models.AcquisitionFilter
	acqFilter.AcqUID = acqID
	acqFilter.RequiredStateForAtLeastOneTile = models.TileCreateState
	acqCheck, err := h.imageCatcher.GetAcquisitions(&acqFilter)
	if len(acqCheck) > 0 {
		incompleteTilesMsg := fmt.Sprintf("Acq %d marked as complete but it may still have tiles in %s state", acqID, models.TileCreateState)
		logger.Error(incompleteTilesMsg)
		h.messageNotifier.SendMessage(incompleteTilesMsg, true)
	}
}

func (h *httpServerHandler) storeAncillaryFiles(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var acqID uint64
	var err error

	w.Header().Set("Content-Type", "application/json")

	if acqID, err = parseRequiredParamValueAsUint64("acqid", params.ByName("acqid")); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	acqMosaic, err := h.imageCatcher.GetMosaic(acqID)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	h.processAncillaryFiles(w, r, acqMosaic)
}

func (h *httpServerHandler) processAncillaryFiles(w http.ResponseWriter, r *http.Request, acqMosaic *models.TemImageMosaic) {
	err := r.ParseMultipartForm(1 << 20)
	if err != nil {
		if err == http.ErrNotMultipart && len(r.PostForm) == 0 && r.MultipartForm == nil {
			// if this is the case simply return for now
			logger.Infof("Not a multipart/form encoded request")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		}
		parseReqErr := fmt.Errorf("Error parsing the request: %v", err)
		logger.Error(parseReqErr)
		writeError(w, parseReqErr, http.StatusBadRequest)
		return
	}
	fhs := r.MultipartForm.File["ancillary-file"]
	var afResults []map[string]interface{}
	af := &FileParams{}
	var status int
	for _, fh := range fhs {
		var fbytes []byte
		afResult := map[string]interface{}{
			"name": fh.Filename,
		}
		afResults = append(afResults, afResult)
		f, err := fh.Open()
		if err != nil {
			afErr := fmt.Errorf("Error opening the ancillary file %s: %v", fh.Filename, err)
			logger.Error(afErr)
			afResult["errormessage"] = afErr.Error()
			status = http.StatusBadRequest
			continue
		}
		fbytes, err = ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			afErr := fmt.Errorf("Error reading the ancillary file %s: %v", fh.Filename, err)
			logger.Error(afErr)
			afResult["errormessage"] = afErr.Error()
			status = http.StatusBadRequest
			continue
		}
		if len(fbytes) == 0 {
			afErr := fmt.Errorf("Empty content encountered for %s", fh.Filename)
			logger.Error(afErr)
			afResult["errormessage"] = afErr.Error()
			status = http.StatusBadRequest
			continue
		}
		af.Content = fbytes
		af.Name = fh.Filename
		af.ContentLen = len(fbytes)
		fObj, err := h.imageCatcher.CreateAncillaryFile(acqMosaic, af)
		af.reset()
		if err != nil {
			afErr := fmt.Errorf("Error storing the ancillary file %s: %v", fh.Filename, err)
			logger.Error(afErr)
			afResult["errormessage"] = afErr.Error()
			status = http.StatusBadRequest
		} else {
			afResult["id"] = fObj.FileObjectID
			afResult["path"] = fObj.Path
			afResult["jfs_path"] = fObj.JfsPath
			afResult["jfs_key"] = fObj.JfsKey
		}
	}
	if status == 0 {
		status = http.StatusOK
	}
	result := map[string]interface{}{
		"af_results": afResults,
	}
	respBytes, merr := json.Marshal(result)
	if merr != nil {
		internalErr := fmt.Errorf("Error marshalling the result %v: %v", result, merr)
		logger.Error(internalErr)
		writeError(w, internalErr, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(respBytes)
}

// extractFileField - extracts the file field from the multipart if the multipart form fieldname matches the provided one
// it returns a boolean and an error where the boolean is true if the fieldname matches, false otherwise and the error
// is the returned in case the fieldname matches but there was an error while extracting the field content
func extractFileField(p *multipart.Part, fieldName string, f *FileParams) (bool, error) {
	currentFieldName := p.FormName()
	if currentFieldName == fieldName {
		bytes, err := ioutil.ReadAll(p)
		if err != nil {
			return true, fmt.Errorf("Error reading the %s from the multipart buffer: %v", fieldName, err)
		}
		f.Name = p.FileName()
		f.Content = bytes
		f.ContentLen = len(bytes)
		return true, nil
	}
	return false, nil
}

func (h *httpServerHandler) createROIs(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	acqID, err := strconv.ParseUint(params.ByName("acqid"), 10, 64)
	if err != nil {
		acqIDParseErr := fmt.Errorf("Error while parsing the acqId parameter %v", err)
		logger.Error(acqIDParseErr)
		writeError(w, acqIDParseErr, http.StatusBadRequest)
		return
	}

	formReader, err := r.MultipartReader()
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	roiFiles := &ROIFiles{}
	for {
		p, err := formReader.NextPart()
		if err == io.EOF {
			break
		}
		if err = extractROIs(p, roiFiles); err != nil {
			logger.Error(err)
			writeError(w, err, http.StatusBadRequest)
			return
		}
	}
	acqMosaic, err := h.imageCatcher.GetMosaic(acqID)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	err = h.imageCatcher.CreateROIs(acqMosaic, roiFiles)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

func extractROIs(p *multipart.Part, rf *ROIFiles) error {
	fieldName := p.FormName()
	switch fieldName {
	case "roi-tiles":
		bytes, err := ioutil.ReadAll(p)
		if err != nil {
			return fmt.Errorf("Error reading the ROI Tiles multipart file buffer: %s\n", err)
		}
		rf.RoiTiles.Name = p.FileName()
		rf.RoiTiles.Content = bytes
		rf.RoiTiles.ContentLen = len(bytes)
		return nil
	case "roi-spec":
		bytes, err := ioutil.ReadAll(p)
		if err != nil {
			return fmt.Errorf("Error reading the ROI Spec multipart file buffer: %s\n", err)
		}
		rf.RoiSpec.Name = p.FileName()
		rf.RoiSpec.Content = bytes
		rf.RoiSpec.ContentLen = len(bytes)
		return nil
	default:
		return nil
	}
}

func (h *httpServerHandler) captureImage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	startTime := time.Now()
	logger.Debugf("Start processing %s", r.URL)
	w.Header().Set("Content-Type", "application/json")

	acqID, err := strconv.ParseUint(params.ByName("acqid"), 10, 64)
	if err != nil {
		acqIDParseErr := fmt.Errorf("Error while parsing the acqId parameter %v", err)
		logger.Error(acqIDParseErr)
		writeError(w, acqIDParseErr, http.StatusBadRequest)
		return
	}

	tileProcessingJob := &tileProcessingJob{
		acqID: acqID,
	}
	tileProcessingJob.execCtx.StartTime = startTime
	defer tileProcessingJob.clear()

	err = extractTileJobParams(r, tileProcessingJob)
	if err != nil {
		logger.Errorf("Error extracting the tile parameters (%v): %v", time.Since(startTime), err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	logger.Infof("End request parsing for %d:%s (%d)  %v",
		tileProcessingJob.acqID, tileProcessingJob.tileParams.Name, tileProcessingJob.tileParams.ContentLen, time.Since(startTime))

	tileInfo, err := h.trh.startTileProcessingJob(tileProcessingJob)
	if err != nil {
		logger.Errorf("Error processing the tile parameters for %d:%s (%d)  %v: %v",
			tileProcessingJob.acqID, tileProcessingJob.tileParams.Name,
			tileProcessingJob.tileParams.ContentLen, time.Since(startTime), err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	h.writeQueueStatus(w)
	json.NewEncoder(w).Encode(formatCapturedImageResponse(tileInfo))
	logger.Infof("End captureImage for %d:%s (%d)  %v",
		tileProcessingJob.acqID, tileProcessingJob.tileParams.Name, tileProcessingJob.tileParams.ContentLen, time.Since(startTime))
}

func extractTileJobParams(r *http.Request, tileJobParams *tileProcessingJob) error {
	extractors := []func(*http.Request, *tileProcessingJob) error{
		extractTileJobParamsFromMPRequest,
		extractTileJobParamsFromRequestBody,
	}
	for _, extractor := range extractors {
		err := extractor(r, tileJobParams)
		if err == errWrongExtractMethod {
			continue
		}
		// otherwise simply assume the extractor is supported and return whatever the extractor returned
		return err
	}
	return errWrongExtractMethod
}

func extractTileJobParamsFromMPRequest(r *http.Request, tileJobParams *tileProcessingJob) error {
	formReader, err := r.MultipartReader()
	if err != nil {
		if err == http.ErrNotMultipart {
			logger.Info("Not using multipart encoding for the acquisition log")
			return errWrongExtractMethod
		}
		return err
	}

	for {
		p, err := formReader.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = extractTileParam(p, tileJobParams); err != nil {
			return err
		}
	}
}

func extractTileParam(p *multipart.Part, tileJobParams *tileProcessingJob) error {
	startTime := time.Now()
	var err error

	fieldName := p.FormName()

	switch fieldName {
	case "tile-filename":
		var formFieldBuf bytes.Buffer
		if _, err = io.Copy(&formFieldBuf, p); err != nil {
			return fmt.Errorf("Error extracting the tile file name: %v", err)
		}
		tileFileName := formFieldBuf.String()
		if err = tileJobParams.tileParams.ExtractTileParamsFromURL(tileFileName); err != nil {
			return err
		}
		logger.Debugf("Extracted tile name %s %v", tileJobParams.tileParams.Name, time.Since(startTime))
		return nil
	case "checksum":
		formFieldBuf := new(bytes.Buffer)
		if _, err = io.Copy(formFieldBuf, p); err != nil {
			return fmt.Errorf("Error extracting checksum: %v", err)
		}
		tileJobParams.tileParams.Checksum = formFieldBuf.Bytes()
		return nil
	case "checksum-str": // this expects the checksum to come hex encoded
		var formFieldBuf bytes.Buffer
		if _, err = io.Copy(&formFieldBuf, p); err != nil {
			return fmt.Errorf("Error extracting checksum: %v", err)
		}
		checksum := formFieldBuf.String()
		if tileJobParams.tileParams.Checksum, err = hex.DecodeString(checksum); err != nil {
			// log the error but continue processing if this is the only problem
			logger.Errorf("Error decoding the checksum %s: %v\n", checksum, err)
		}
		return nil
	case "tile-file":
		formFieldBuf := new(bytes.Buffer)
		if tileJobParams.tileParams.Name == "" {
			fName := p.FileName()
			logger.Debugf("Extract tile params from %s", fName)
			tileJobParams.tileParams.ExtractTileParamsFromURL(fName)
		}
		if _, err = io.Copy(formFieldBuf, p); err != nil {
			return fmt.Errorf("Error reading the multipart file buffer: %v\n", err)
		}
		tileJobParams.tileParams.Content = formFieldBuf.Bytes()
		tileJobParams.tileParams.ContentLen = len(tileJobParams.tileParams.Content)
		logger.Debugf("Extracted tile content %s (%d bytes) %v", tileJobParams.tileParams.Name, tileJobParams.tileParams.ContentLen, time.Since(startTime))
		return nil
	default:
		return nil
	}
}

func extractTileJobParamsFromRequestBody(r *http.Request, tileJobParams *tileProcessingJob) (err error) {
	startTime := time.Now()
	tileFileName := r.URL.Query().Get("tile-filename")
	if tileFileName == "" {
		err = fmt.Errorf("Invalid tile file name")
		logger.Errorf("%s", err)
		return err
	}
	if err = tileJobParams.tileParams.ExtractTileParamsFromURL(tileFileName); err != nil {
		return err
	}
	tileJobParams.tileParams.Name = tileFileName

	checksum := r.URL.Query().Get("checksum")
	if checksum != "" {
		tileJobParams.tileParams.Checksum, err = hex.DecodeString(checksum)
		if err != nil {
			logger.Errorf("Error decoding the checksum %s: %v\n", checksum, err)
		}
	}
	var requestBuffer *bytes.Buffer
	bodyLen := r.ContentLength
	if bodyLen > 0 {
		requestBufferPtr, requestBufferBytes := utils.Alloc(uint32(bodyLen) + bytes.MinRead + 1)
		requestBuffer = bytes.NewBuffer(requestBufferBytes[0:0])
		tileJobParams.memPtr = requestBufferPtr
	} else {
		requestBuffer = bytes.NewBuffer(make([]byte, 0, 1024*1024))
	}
	if _, err = io.Copy(requestBuffer, r.Body); err != nil {
		return fmt.Errorf("Error reading the request body: %v", err)
	}
	tileJobParams.tileParams.Content = requestBuffer.Bytes()
	tileJobParams.tileParams.ContentLen = len(tileJobParams.tileParams.Content)
	logger.Debugf("Extracted tile content %s (%d) %v", tileJobParams.tileParams.Name, tileJobParams.tileParams.ContentLen, time.Since(startTime))
	return nil
}

func formatCapturedImageResponse(ti *models.TemImage) map[string]interface{} {
	r := make(map[string]interface{})
	r["tile_acq_id"] = ti.ImageMosaic.AcqUID
	r["tile_temca"] = ti.ImageMosaic.Temca.TemcaID
	r["tile_id"] = ti.ImageID
	r["tile_col"] = ti.Col
	r["tile_row"] = ti.Row
	if ti.Configuration != nil {
		r["tile_camera"] = ti.Configuration.Camera
		r["tile_frame"] = ti.Frame
	}
	return r
}

func formatTileImageResponse(ti *models.TemImageROI) map[string]interface{} {
	r := make(map[string]interface{})
	r["tile_acq_id"] = ti.ImageMosaic.AcqUID
	r["tile_temca"] = ti.ImageMosaic.Temca.TemcaID
	r["tile_id"] = ti.ImageID
	r["tile_col"] = ti.Col
	r["tile_row"] = ti.Row
	if ti.Configuration != nil {
		r["tile_camera"] = ti.Configuration.Camera
		r["tile_frame"] = ti.Frame
	}
	r["tile_jfs_path"] = ti.TileFile.JfsPath
	r["tile_jfs_key"] = ti.TileFile.JfsKey
	r["tile_file_checksum"] = hex.EncodeToString(ti.TileFile.Checksum)
	r["image_url"] = ti.TileFile.LocationURL
	if ti.AcquiredTimestamp != nil {
		// the time comes in UTC but for display we convert it to local time
		localAcquiredTimestamp := ti.AcquiredTimestamp.In(timeZone)
		r["tile_acquired"] = localAcquiredTimestamp.Format("20060102150405.000000")
	} else {
		r["tile_acquired"] = nil
	}
	r["state"] = ti.State
	r["section"] = ti.Roi.NominalSection
	return r
}

func (h *httpServerHandler) getAcquisitionProjects(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		filter models.ProjectFilter
		err    error
	)
	w.Header().Set("Content-Type", "application/json")
	if filter.Pagination.StartRecordIndex, err = getQueryParamAsInt64(r, "offset", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if filter.Pagination.NRecords, err = getQueryParamAsInt32(r, "length", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	filter.ProjectName = getQueryParamAsString(r, "project", "")
	filter.ProjectOwner = getQueryParamAsString(r, "owner", "")
	filter.SampleName = getQueryParamAsString(r, "sample", "")
	filter.StackName = getQueryParamAsString(r, "stack", "")
	filter.MosaicType = getQueryParamAsString(r, "mosaic-type", "")
	if filter.DataAcquiredInterval.From, err = getQueryParamAsTime(r, "has-acq-from", "", "20060102150405", "200601021504", "2006010215", "20060102"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if filter.DataAcquiredInterval.To, err = getQueryParamAsTime(r, "has-acq-to", "", "20060102150405", "200601021504", "2006010215", "20060102"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	projects, err := h.imageCatcher.GetProjects(&filter)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	resp := make(map[string]interface{})
	resp["length"] = len(projects)
	if len(projects) == 0 {
		resp["projects"] = []*models.Project{}
	} else {
		resp["projects"] = projects
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *httpServerHandler) getAcquisitions(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		acqFilter models.AcquisitionFilter
		err       error
	)
	w.Header().Set("Content-Type", "application/json")
	if acqFilter.AcqUID, err = getQueryParamAsUint64(r, "acqid", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if acqFilter.Pagination.StartRecordIndex, err = getQueryParamAsInt64(r, "offset", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if acqFilter.Pagination.NRecords, err = getQueryParamAsInt32(r, "length", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	acqFilter.ProjectName = getQueryParamAsString(r, "project", "")
	acqFilter.ProjectOwner = getQueryParamAsString(r, "owner", "")
	acqFilter.SampleName = getQueryParamAsString(r, "sample", "")
	acqFilter.StackName = getQueryParamAsString(r, "stack", "")
	acqFilter.MosaicType = getQueryParamAsString(r, "mosaic-type", "")
	acqFilter.RequiredStateForAtLeastOneTile = getQueryParamAsString(r, "exists-tile-in-state", "")
	acqFilter.RequiredStateForAllTiles = getQueryParamAsString(r, "all-tiles-in-state", "")
	if acqFilter.AcquiredInterval.From, err = getQueryParamAsTime(r, "acq-from", "", "20060102150405", "200601021504", "2006010215", "20060102"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if acqFilter.AcquiredInterval.To, err = getQueryParamAsTime(r, "acq-to", "", "20060102150405", "200601021504", "2006010215", "20060102"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	acquisitions, err := h.imageCatcher.GetAcquisitions(&acqFilter)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	acqResponse := make(map[string]interface{})
	acqResponse["length"] = len(acquisitions)
	if len(acquisitions) == 0 {
		acqResponse["acquisitions"] = []*models.Acquisition{}
	} else {
		acqResponse["acquisitions"] = acquisitions
	}
	json.NewEncoder(w).Encode(acqResponse)
}

func (h *httpServerHandler) getAllTiles(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileFilter models.TileFilter
		err        error
	)
	w.Header().Set("Content-Type", "application/json")

	if tileFilter.AcqUID, err = getQueryParamAsUint64(r, "acqid", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileFilter.ProjectName = getQueryParamAsString(r, "project", "")
	tileFilter.ProjectOwner = getQueryParamAsString(r, "owner", "")
	tileFilter.SampleName = getQueryParamAsString(r, "sample", "")
	tileFilter.StackName = getQueryParamAsString(r, "stack", "")
	tileFilter.MosaicType = getQueryParamAsString(r, "mosaic-type", "")
	tileFilter.State = getQueryParamAsString(r, "tile-state", "")
	if tileFilter.TileAcquiredInterval.From, err = getQueryParamAsTime(r, "tile-from", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.TileAcquiredInterval.To, err = getQueryParamAsTime(r, "tile-to", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Col, err = getQueryParamAsInt(r, "col", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Row, err = getQueryParamAsInt(r, "row", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.CameraConfig.Camera, err = getQueryParamAsInt(r, "camera", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileFilter.FrameType.Set(getQueryParamAsString(r, "frame-type", ""))
	if tileFilter.Pagination.StartRecordIndex, err = getQueryParamAsInt64(r, "offset", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Pagination.NRecords, err = getQueryParamAsInt32(r, "length", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.IncludeNotPersisted, err = getQueryParamAsBool(r, "include-all", false); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	tiles, err := h.imageCatcher.GetTiles(&tileFilter)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	h.encodeTileListResponse(w, tiles)
}

// tilesByAcqUIDAndTimestamp implements a sort.Interface for sorting an array of tiles
// by acquisition id and timestamp
type tilesByAcqUIDAndTimestamp []*models.TemImageROI

// Len length of the array
func (ta tilesByAcqUIDAndTimestamp) Len() int {
	return len(ta)
}

// Swap two members of the array
func (ta tilesByAcqUIDAndTimestamp) Swap(i, j int) {
	ta[i], ta[j] = ta[j], ta[i]
}

// Less compares two members of the array
func (ta tilesByAcqUIDAndTimestamp) Less(i, j int) bool {
	if ta[i].ImageMosaic.AcqUID == ta[j].ImageMosaic.AcqUID {
		if ta[i].AcquiredTimestamp == nil && ta[j].AcquiredTimestamp == nil {
			return true
		}
		if ta[i].AcquiredTimestamp == nil {
			return false
		}
		if ta[j].AcquiredTimestamp == nil {
			return true
		}
		return ta[i].AcquiredTimestamp.Before(*ta[j].AcquiredTimestamp)
	}
	return ta[i].ImageMosaic.AcqUID < ta[j].ImageMosaic.AcqUID
}

func (h *httpServerHandler) encodeTileListResponse(w http.ResponseWriter, tiles []*models.TemImageROI) {
	tilesResponse := make(map[string]interface{})

	var currentAcq map[string]interface{}
	var acqList, tileList []map[string]interface{}
	var prevTile *models.TemImageROI
	sort.Stable(tilesByAcqUIDAndTimestamp(tiles))
	for _, t := range tiles {
		if prevTile == nil || prevTile.ImageMosaic.AcqUID != t.ImageMosaic.AcqUID {
			if prevTile != nil {
				currentAcq["tiles"] = tileList
				currentAcq["n_tiles"] = len(tileList)
				tileList = make([]map[string]interface{}, 0)
			}
			currentAcq = make(map[string]interface{})
			currentAcq["acq_id"] = t.ImageMosaic.AcqUID
			currentAcq["acq_timestamp"] = t.ImageMosaic.Acquired
			currentAcq["acq_completed"] = t.ImageMosaic.Completed
			acqList = append(acqList, currentAcq)
		}
		tileList = append(tileList, formatTileImageResponse(t))
		prevTile = t
	}
	// set the tiles for the last acquisition
	if prevTile != nil {
		currentAcq["tiles"] = tileList
		currentAcq["n_tiles"] = len(tileList)
	}
	tilesResponse["n_returned_acqs"] = len(acqList)
	if len(acqList) == 0 {
		tilesResponse["acquisitions"] = []map[string]interface{}{}
	} else {
		tilesResponse["acquisitions"] = acqList
	}
	tilesResponse["n_returned_tiles_for_all_acqs"] = len(tiles)
	if err := json.NewEncoder(w).Encode(tilesResponse); err != nil {
		logger.Error("Error encoding tiles as JSON", tiles, err)
	}
	return
}

func (h *httpServerHandler) getAcqTiles(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileFilter models.TileFilter
		err        error
	)
	w.Header().Set("Content-Type", "application/json")

	if tileFilter.AcqUID, err = parseRequiredParamValueAsUint64("acqid", params.ByName("acqid")); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileFilter.State = getQueryParamAsString(r, "tile-state", "")
	if tileFilter.TileAcquiredInterval.From, err = getQueryParamAsTime(r, "tile-from", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.TileAcquiredInterval.To, err = getQueryParamAsTime(r, "tile-to", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Col, err = getQueryParamAsInt(r, "col", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Row, err = getQueryParamAsInt(r, "row", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.CameraConfig.Camera, err = getQueryParamAsInt(r, "camera", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileFilter.FrameType.Set(getQueryParamAsString(r, "frame-type", ""))
	if tileFilter.Pagination.StartRecordIndex, err = getQueryParamAsInt64(r, "offset", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Pagination.NRecords, err = getQueryParamAsInt32(r, "length", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.IncludeNotPersisted, err = getQueryParamAsBool(r, "include-all", false); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	tiles, err := h.imageCatcher.GetTiles(&tileFilter)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	h.encodeTileListResponse(w, tiles)
}

func getQueryParamAsInt(r *http.Request, paramName string, defValue int) (int, error) {
	paramValue := r.URL.Query().Get(paramName)
	return parseParamValueAsInt(paramName, paramValue, defValue)
}

func getQueryParamAsInt32(r *http.Request, paramName string, defValue int32) (val int32, err error) {
	paramValue := r.URL.Query().Get(paramName)
	if len(paramValue) != 0 {
		var val64 int64
		var valErr error
		if val64, valErr = strconv.ParseInt(paramValue, 10, 32); valErr != nil {
			err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
			val = defValue
		} else {
			val = int32(val64)
		}
	} else {
		val = defValue
	}
	return
}

func getQueryParamAsInt64(r *http.Request, paramName string, defValue int64) (int64, error) {
	paramValue := r.URL.Query().Get(paramName)
	return parseParamValueAsInt64(paramName, paramValue, defValue)
}

func getQueryParamAsUint64(r *http.Request, paramName string, defValue uint64) (uint64, error) {
	paramValue := r.URL.Query().Get(paramName)
	return parseParamValueAsUint64(paramName, paramValue, defValue)
}

func getQueryParamAsString(r *http.Request, paramName string, defValue string) string {
	paramValue := r.URL.Query().Get(paramName)
	if len(paramValue) == 0 {
		paramValue = defValue
	}
	return paramValue
}

func getQueryParamAsTime(r *http.Request, paramName string, defValue string, patterns ...string) (*time.Time, error) {
	paramValue := getQueryParamAsString(r, paramName, defValue)
	if paramValue == "" {
		return nil, nil
	}
	for _, pattern := range patterns {
		// parse the time in local time zone but them convert it to UTC because the time is always persisted in UTC
		t, err := time.ParseInLocation(pattern, paramValue, timeZone)
		if err == nil {
			utcTime := t.In(utcTimeZone)
			logger.Debugf("Successfully parsed time value '%s' of '%s' using pattern '%s' as %v (%v)", paramValue, paramName, pattern, t, utcTime)
			return &utcTime, nil
		}
		logger.Debugf("Pattern '%s' failed for parsing value '%s' of '%s': %v", pattern, paramValue, paramName, err)
	}
	return nil, fmt.Errorf("Could not parse time value '%s' of '%s' with any of the %v patterns", paramValue, paramName, patterns)
}

func getQueryParamAsBool(r *http.Request, paramName string, defValue bool) (val bool, err error) {
	paramValue := r.URL.Query().Get(paramName)
	if len(paramValue) != 0 {
		var valErr error
		if val, valErr = strconv.ParseBool(paramValue); valErr != nil {
			err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
			val = defValue
		}
	} else {
		val = defValue
	}
	return
}

func parseRequiredParamValueAsUint64(paramName, paramValue string) (val uint64, err error) {
	var valErr error
	if val, valErr = strconv.ParseUint(paramValue, 10, 64); valErr != nil {
		err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
	}
	return
}

func parseRequiredParamValueAsInt64(paramName, paramValue string) (val int64, err error) {
	var valErr error
	if val, valErr = strconv.ParseInt(paramValue, 10, 64); valErr != nil {
		err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
	}
	return
}

func parseParamValueAsInt(paramName, paramValue string, defValue int) (val int, err error) {
	if len(paramValue) != 0 {
		var valErr error
		if val, valErr = strconv.Atoi(paramValue); valErr != nil {
			err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
			val = defValue
		}
	} else {
		val = defValue
	}
	return
}

func parseParamValueAsInt64(paramName, paramValue string, defValue int64) (val int64, err error) {
	if len(paramValue) != 0 {
		var valErr error
		if val, valErr = strconv.ParseInt(paramValue, 10, 64); valErr != nil {
			err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
			val = defValue
		}
	} else {
		val = defValue
	}
	return
}

func parseParamValueAsUint64(paramName, paramValue string, defValue uint64) (val uint64, err error) {
	if len(paramValue) != 0 {
		var valErr error
		if val, valErr = strconv.ParseUint(paramValue, 10, 64); valErr != nil {
			err = fmt.Errorf("Error while parsing the %s parameter - %s: %v", paramName, paramValue, valErr)
			val = defValue
		}
	} else {
		val = defValue
	}
	return
}

func (h *httpServerHandler) getTileByTileID(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileID int64
		err    error
	)
	w.Header().Set("Content-Type", "application/json")

	if tileID, err = parseRequiredParamValueAsInt64("tileid", params.ByName("tileid")); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileInfo, err := h.imageCatcher.GetTile(tileID)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	if tileInfo == nil {
		logger.Infof("No tile found for %d", tileID)
		writeError(w, err, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(formatTileImageResponse(tileInfo))
}

func (h *httpServerHandler) getTileContentByTileID(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileID int64
		err    error
	)
	w.Header().Set("Content-Type", "image/tiff")

	if tileID, err = parseRequiredParamValueAsInt64("tileid", params.ByName("tileid")); err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tileInfo, err := h.imageCatcher.GetTile(tileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tileInfo == nil {
		logger.Infof("No tile found for %d", tileID)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	tileContent, err := h.imageCatcher.RetrieveAcquisitionFile(&tileInfo.ImageMosaic, tileInfo.TileFile.JfsPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(tileContent)
}

func (h *httpServerHandler) getTileByTileCoord(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileFilter models.TileFilter
		err        error
	)
	w.Header().Set("Content-Type", "application/json")

	if tileFilter.AcqUID, err = parseRequiredParamValueAsUint64("acqid", params.ByName("acqid")); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Col, err = parseParamValueAsInt("col", params.ByName("col"), -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if tileFilter.Row, err = parseParamValueAsInt("row", params.ByName("row"), -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	tileFilter.CameraConfig.Camera = -1
	tileFilter.Pagination.StartRecordIndex = 0
	tileFilter.Pagination.NRecords = 2
	tiles, err := h.imageCatcher.GetTiles(&tileFilter)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if len(tiles) == 0 {
		err = fmt.Errorf("No tile found for %v", tileFilter)
		logger.Error(err)
		writeError(w, err, http.StatusNotFound)
		return
	}
	if len(tiles) > 1 {
		err = fmt.Errorf("More than one frame found for %v", tileFilter)
		logger.Error(err)
		writeError(w, err, http.StatusConflict)
		return
	}
	tileInfo := tiles[0]
	if tileInfo == nil {
		logger.Infof("No tile found for %v", tileFilter)
		err = fmt.Errorf("No tile found for %v", tileFilter)
		writeError(w, err, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(formatTileImageResponse(tileInfo))
}

func (h *httpServerHandler) getTileContentByTileCoord(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileFilter models.TileFilter
		err        error
	)
	w.Header().Set("Content-Type", "image/tiff")

	if tileFilter.AcqUID, err = parseRequiredParamValueAsUint64("acqid", params.ByName("acqid")); err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if tileFilter.Col, err = parseParamValueAsInt("col", params.ByName("col"), -1); err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if tileFilter.Row, err = parseParamValueAsInt("row", params.ByName("row"), -1); err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tileFilter.CameraConfig.Camera = -1
	tileFilter.Pagination.StartRecordIndex = 0
	tileFilter.Pagination.NRecords = 2
	tiles, err := h.imageCatcher.GetTiles(&tileFilter)
	if err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(tiles) == 0 {
		err = fmt.Errorf("No tile found for %v", tileFilter)
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if len(tiles) > 1 {
		err = fmt.Errorf("More than one frame found for %v", tileFilter)
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	tileInfo := tiles[0]
	if tileInfo == nil {
		err = fmt.Errorf("No tile found for %v", tileFilter)
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if tileInfo.TileFile.JfsPath == "" || tileInfo.TileFile.JfsKey == "" {
		err = fmt.Errorf("Tile metadata found but no content found for %v", tileFilter)
		logger.Error(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	tileContent, err := h.imageCatcher.RetrieveAcquisitionFile(&tileInfo.ImageMosaic, tileInfo.TileFile.JfsPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(tileContent)
}

func (h *httpServerHandler) verifyTile(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		tileID int64
		err    error
	)
	w.Header().Set("Content-Type", "application/json")

	if tileID, err = strconv.ParseInt(params.ByName("tileid"), 10, 64); err != nil {
		tileIDParseErr := fmt.Errorf("Error while parsing the tileId parameter %v", err)
		logger.Error(tileIDParseErr)
		writeError(w, tileIDParseErr, http.StatusBadRequest)
		return
	}
	checksumAlg := r.URL.Query().Get("checksum-algorithm")

	tileInfo, err := h.imageCatcher.GetTile(tileID)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	if tileInfo == nil {
		logger.Infof("No tile found for %d", tileID)
		writeError(w, err, http.StatusNotFound)
		return
	}
	tileResponse := formatTileImageResponse(tileInfo)
	if tileInfo.TileFile.JfsPath == "" {
		tileInfoErr := fmt.Errorf("JFSPath not set for %d", tileID)
		logger.Error(tileInfoErr)
		w.WriteHeader(http.StatusExpectationFailed)
		tileResponse["errormessage"] = tileInfoErr.Error()
		json.NewEncoder(w).Encode(tileResponse)
		return
	}
	tileContent, err := h.imageCatcher.RetrieveAcquisitionFile(&tileInfo.ImageMosaic, tileInfo.TileFile.JfsPath)
	if err != nil {
		tileContentErr := fmt.Errorf("Error while retrieving content for %s: %v", tileInfo.TileFile.JfsPath, err)
		logger.Error(tileContentErr)
		w.WriteHeader(http.StatusExpectationFailed)
		tileResponse["errormessage"] = tileContentErr.Error()
		json.NewEncoder(w).Encode(tileResponse)
		return
	}
	var checksum []byte
	switch checksumAlg {
	case "SHA1":
		sha1Checksum := sha1.Sum(tileContent)
		checksum = sha1Checksum[0:]
	default:
		checksumAlg = "MD5"
		md5Checksum := md5.Sum(tileContent)
		checksum = md5Checksum[0:]
	}
	tileResponse["calculatedChecksum"] = checksum
	if bytes.Compare(checksum, tileInfo.TileFile.Checksum) != 0 {
		err := fmt.Errorf("%s checksum stored does not match the calculated checksum - expected %x but got %x",
			checksumAlg, checksum, tileInfo.TileFile.Checksum)
		logger.Error(err)
		w.WriteHeader(http.StatusExpectationFailed)
		tileResponse["errormessage"] = err.Error()
		json.NewEncoder(w).Encode(tileResponse)
		return
	}
	json.NewEncoder(w).Encode(formatTileImageResponse(tileInfo))
}

func (h *httpServerHandler) createCalibrations(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	var cr CalibrationJSONReader
	calibrations, err := cr.Read(r.Body)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}

	err = h.configurator.ImportCalibrations(calibrations)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	h.encodeCalibrationsResponse(w, calibrations)
}

func (h *httpServerHandler) encodeCalibrationsResponse(w http.ResponseWriter, calibrations []*models.Calibration) {
	resp := make(map[string]interface{})
	var calibrationList []map[string]interface{}
	for _, c := range calibrations {
		calibration := encodeCalibration(c)
		calibrationList = append(calibrationList, calibration)
	}

	resp["length"] = len(calibrations)
	if len(calibrationList) == 0 {
		calibrationList = []map[string]interface{}{}
	}
	resp["calibrations"] = calibrationList

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error(err)
	}
}

func encodeCalibration(c *models.Calibration) map[string]interface{} {
	var calibrationContent map[string]interface{}
	calibration := make(map[string]interface{})
	calibration["id"] = c.ID
	calibration["name"] = c.Name
	calibration["calibration_date"] = c.Generated.Format("20060102")
	decoder := json.NewDecoder(strings.NewReader(c.JSONContent))
	if err := decoder.Decode(&calibrationContent); err != nil {
		logger.Error(err)
		calibration["content_errormessage"] = err.Error()
	} else {
		calibration["content"] = calibrationContent
	}
	return calibration
}

func (h *httpServerHandler) getCalibrations(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		err        error
		pagination models.Page
	)
	w.Header().Set("Content-Type", "application/json")

	if pagination.StartRecordIndex, err = getQueryParamAsInt64(r, "offset", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if pagination.NRecords, err = getQueryParamAsInt32(r, "length", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	calibrations, err := h.configurator.GetCalibrations(pagination)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	h.encodeCalibrationsResponse(w, calibrations)
}

func (h *httpServerHandler) getCalibrationsByName(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var err error
	w.Header().Set("Content-Type", "application/json")

	calibrationName := params.ByName("calibration_name")
	if calibrationName == "" {
		err = fmt.Errorf("Invalid calibration_name")
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	calibrations, err := h.configurator.GetCalibrationsByName(calibrationName)
	if err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	if len(calibrations) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{}"))
		return
	}
	if err := json.NewEncoder(w).Encode(encodeCalibration(calibrations[0])); err != nil {
		logger.Error(err)
	}
}

func (h *httpServerHandler) serveNextTiles(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var (
		filter models.TileFilter
		err    error
	)
	w.Header().Set("Content-Type", "application/json")

	if filter.AcqUID, err = getQueryParamAsUint64(r, "acqid", 0); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if filter.NominalSection, err = getQueryParamAsInt64(r, "section", -1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if filter.Pagination.NRecords, err = getQueryParamAsInt32(r, "ntiles", 1); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	filter.ProjectName = getQueryParamAsString(r, "project", "")
	filter.SampleName = getQueryParamAsString(r, "sample", "")
	filter.StackName = getQueryParamAsString(r, "stack", "")
	filter.MosaicType = getQueryParamAsString(r, "mosaic-type", "")
	filter.State = getQueryParamAsString(r, "oldState", models.TileReadyState)
	if filter.TileAcquiredInterval.From, err = getQueryParamAsTime(r, "tile-from", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if filter.TileAcquiredInterval.To, err = getQueryParamAsTime(r, "tile-to", "", "20060102150405.000000", "20060102150405.000", "20060102150405"); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	filter.FrameType.Set(getQueryParamAsString(r, "frame-type", ""))

	nextTileState := getQueryParamAsString(r, "newState", models.TileInProgressState)

	// select the next available tile
	tilespecs, resultType, err := h.tileDistributor.ServeNextAvailableTiles(&filter, nextTileState)
	tilespecResp := prepareTileSpecResult(filter.AcqUID, filter.NominalSection, tilespecs, resultType, filter.Pagination.NRecords)
	var retStatus int
	if err != nil {
		logger.Errorf("Error retrieving the next tile spec: %v", err)
		retStatus = http.StatusInternalServerError
		tilespecResp["errormessage"] = err.Error()
	} else {
		retStatus = http.StatusOK
	}
	w.WriteHeader(retStatus)
	tilespecBytes, werr := encodeTileSpec(tilespecResp)
	if werr != nil {
		logger.Errorf("Error encoding response %v: %v", tilespecResp, werr)
		return
	}
	if _, werr := w.Write(tilespecBytes); werr != nil {
		logger.Errorf("Error writing response %s: %v", string(tilespecBytes), werr)
	}
}

func encodeTileSpec(tilespec map[string]interface{}) ([]byte, error) {
	return json.Marshal(tilespec)
}

func prepareTileSpecResult(reqAcqID uint64, reqSection int64, tileResults []*models.TileSpec, resultType SelectTileResultType, ntiles int32) map[string]interface{} {
	r := make(map[string]interface{})
	r["resultType"] = resultType.String()
	if reqAcqID > 0 {
		r["requestedAcqId"] = reqAcqID
	}
	if reqSection >= 0 {
		r["requestedSection"] = reqSection
	}
	r["nTilesRequested"] = ntiles
	r["nTilesFound"] = len(tileResults)

	var tilespecs []map[string]interface{}
	if len(tileResults) == 0 {
		r["results"] = []map[string]interface{}{}
		return r
	}
	for _, tilespec := range tileResults {
		tilespecs = append(tilespecs, prepareTileSpec(tilespec))
	}
	r["results"] = tilespecs
	return r
}

func prepareTileSpec(tileResult *models.TileSpec) map[string]interface{} {
	r := make(map[string]interface{})
	var stageX, stageY float64 // (Center of stage in pixel units) + (half distance of montage in pixel units) + (actual stage motion to this point)

	switch tileResult.NumberOfCameras {
	case 1:
		stageX = -(tileResult.XCenter * 1.0E6 * tileResult.PixPerUm) -
			(float64(tileResult.XSmallStepPix) * float64(tileResult.NXSteps) / 2.0) +
			float64(tileResult.Col)*float64(tileResult.XSmallStepPix)
		stageY = ((tileResult.YCenter + tileResult.YTem) * 1.0E6 * tileResult.PixPerUm) -
			(float64(tileResult.YSmallStepPix) * float64(tileResult.NYSteps) / 2.0) +
			float64(tileResult.Row)*float64(tileResult.YSmallStepPix)
	case 4:
		stageX = -(tileResult.XCenter * 1.0E6 * tileResult.PixPerUm) -
			((float64(tileResult.XBigStepPix) + float64(tileResult.XSmallStepPix)) * (float64(tileResult.NXSteps) / 4.0)) +
			((float64(tileResult.XBigStepPix) + float64(tileResult.XSmallStepPix)) * (float64(tileResult.Col) / 4)) +
			(float64(tileResult.XSmallStepPix) * float64(tileResult.Col%4))
		stageY = ((tileResult.YCenter + tileResult.YTem) * 1.0E6 * tileResult.PixPerUm) -
			((float64(tileResult.YBigStepPix) + float64(tileResult.YSmallStepPix)) * (float64(tileResult.NYSteps) / 4.0)) +
			((float64(tileResult.YBigStepPix) + float64(tileResult.YSmallStepPix)) * (float64(tileResult.Row) / 4)) +
			(float64(tileResult.YSmallStepPix) * float64(tileResult.Row%4))
	}
	var (
		sectionID string
		z         float64
		err       error
	)
	sectionID = fmt.Sprintf("%d.%d", tileResult.NominalSection, tileResult.PrevAcqCount)
	if z, err = strconv.ParseFloat(sectionID, 64); err != nil {
		// checking this just in case
		logger.Errorf("Error converting the sectionId %s to a float: %v", sectionID, err)
		// if this happens simply set z to the nominal section
		z = float64(tileResult.NominalSection)
	}
	tilespec := map[string]interface{}{
		"tileId": fmt.Sprintf("%d.%03d.%03d.%s", tileResult.AcqUID, tileResult.Col, tileResult.Row, sectionID),
		"width":  tileResult.Width,
		"height": tileResult.Height,
		"z":      z,
		"mipmapLevels": map[string]interface{}{
			"0": map[string]interface{}{
				"imageUrl": tileResult.ImageURL,
				"maskUrl":  tileResult.MaskURL,
			},
		},
		"layout": map[string]interface{}{
			"camera":    tileResult.CameraConfig.Camera,
			"imageCol":  tileResult.Col,
			"imageRow":  tileResult.Row,
			"sectionId": sectionID,
			"temca":     fmt.Sprintf("%d", tileResult.CameraConfig.TemcaID),
			"stageX":    stageX,
			"stageY":    stageY,
			"rotation":  0,
		},
		"transforms": map[string]interface{}{
			"type": "list",
			"specList": []map[string]string{
				map[string]string{
					"type":  "ref",
					"refId": tileResult.TransformationRefID,
				},
				map[string]string{
					"type":       "leaf",
					"className":  "mpicbg.trakem2.transform.AffineModel2D",
					"dataString": fmt.Sprintf("1.0 0.0 0.0 1.0 %d %d", int(stageX), int(stageY)),
				},
			},
		},
	}
	r["acqid"] = tileResult.AcqUID
	r["section"] = tileResult.NominalSection
	r["tilespec"] = tilespec
	return r
}

func (h *httpServerHandler) updateTilesState(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var nUpdates int64
	var err error
	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	var toUpdate struct {
		State       string
		TileSpecIds []string
	}
	if err = decoder.Decode(&toUpdate); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if toUpdate.State == "" {
		err = fmt.Errorf("Invalid tile state - tile state cannot be empty")
		logger.Error(err)
		writeError(w, err, http.StatusBadRequest)
		return
	}
	logger.Infof("Update %v to %v", toUpdate.TileSpecIds, toUpdate.State)
	tilespecs := make([]*models.TileSpec, len(toUpdate.TileSpecIds))
	for i, tileSpecID := range toUpdate.TileSpecIds {
		if tilespecs[i], err = extractTileSpecFromTileSpecID(tileSpecID); err != nil {
			logger.Error(err)
			writeError(w, err, http.StatusBadRequest)
			return
		}
	}
	if nUpdates, err = h.tileDistributor.UpdateTilesState(tilespecs, toUpdate.State); err != nil {
		logger.Error(err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"nUpdates": %d}`, nUpdates)
}

func extractTileSpecFromTileSpecID(tileSpecID string) (*models.TileSpec, error) {
	var (
		acqID         uint64
		section       int64
		prevAcqNumber int
		col, row      int
		err           error
	)
	tileSpecFields := strings.Split(tileSpecID, ".")
	if len(tileSpecFields) != 5 {
		return nil, fmt.Errorf("Invalid tile spec ID format: '%s' - it must <acqID>.<tileCol>.<tileRow>.<tileSection>.<#_of_prev_acquisitions_for_section>", tileSpecID)
	}
	if acqID, err = strconv.ParseUint(tileSpecFields[0], 10, 64); err != nil {
		return nil, fmt.Errorf("Invalid acq ID value %s in %s: %v", tileSpecFields[0], tileSpecID, err)
	}
	if col, err = strconv.Atoi(tileSpecFields[1]); err != nil {
		return nil, fmt.Errorf("Invalid tile column value %s in %s: %v", tileSpecFields[1], tileSpecID, err)
	}
	if row, err = strconv.Atoi(tileSpecFields[2]); err != nil {
		return nil, fmt.Errorf("Invalid tile row value %s in %s: %v", tileSpecFields[2], tileSpecID, err)
	}
	if section, err = strconv.ParseInt(tileSpecFields[3], 10, 64); err != nil {
		return nil, fmt.Errorf("Invalid section value %s in %s: %v", tileSpecFields[3], tileSpecID, err)
	}
	if prevAcqNumber, err = strconv.Atoi(tileSpecFields[4]); err != nil {
		return nil, fmt.Errorf("Invalid value for number of previous acquisitions %s in %s: %v", tileSpecFields[4], tileSpecID, err)
	}
	return &models.TileSpec{
		Acquisition: models.Acquisition{
			AcqUID: acqID,
		},
		Col:            col,
		Row:            row,
		NominalSection: section,
		PrevAcqCount:   prevAcqNumber,
	}, nil
}
