package service

import (
	"fmt"
	"time"

	"imagecatcher/config"
	"imagecatcher/logger"
	"imagecatcher/models"
)

// QueueStatus type for queue status
type QueueStatus int16

const (
	// Green - system is fully functional
	Green QueueStatus = iota
	// Yellow - system is becoming busy
	Yellow
	// Red - system is very close to choking if not already
	Red
)

// String QueueStatus string representation
func (qs QueueStatus) String() string {
	switch qs {
	case Green:
		return "GREEN"
	case Yellow:
		return "YELLOW"
	case Red:
		return "RED"
	default:
		return "INVALID"
	}
}

const defaultRetries int = 3

// TileRequestHandler request handler
type TileRequestHandler struct {
	imageCatcher                ImageCatcherService
	config                      config.Config
	tmpDataDir                  string
	tileProcessingWorkers       int
	tileProcessingDispatcher    *tileProcessingDispatcher
	tileProcessingJobQueue      chan *tileProcessingJob
	contentProcessingWorkers    int
	contentProcessingDispatcher *contentDispatcher
	contentProcessingJobQueue   chan *contentProcessingJob
	messageNotifier             MessageNotifier
	contentProcessedResBuffer   chan contentProcessingResult
	contentProcessingErrors     chan string
	waitTimeout                 time.Duration
	tileProcessor               tileProcessor
	contentProcessor            contentProcessor
}

// NewTileRequestHandler create a new instance of a TileRequestHandler
func NewTileRequestHandler(s ImageCatcherService, messageNotifier MessageNotifier, config config.Config) *TileRequestHandler {
	tp := &tileProcessorImpl{service: s}
	cp := &contentProcessorImpl{service: s}
	th := &TileRequestHandler{
		imageCatcher:     s,
		config:           config,
		messageNotifier:  messageNotifier,
		tileProcessor:    tp,
		contentProcessor: cp,
	}
	th.initialize()
	return th
}

func (th *TileRequestHandler) initialize() {
	var err error
	th.tmpDataDir = th.config.GetStringProperty("TMP_DATA_DIR", "/tmp")
	th.tileProcessingWorkers = th.config.GetIntProperty("TILE_PROCESSING_WORKERS", 1)
	tileProcessingJobQueueSize := th.config.GetIntProperty("TILE_PROCESSING_QUEUE_SIZE", 1)
	th.contentProcessingWorkers = th.config.GetIntProperty("CONTENT_STORE_WORKERS", 1)
	timeout := th.config.GetStringProperty("WAIT_TIMEOUT", "1s")
	if th.waitTimeout, err = time.ParseDuration(timeout); err != nil {
		logger.Errorf("Invalid timeout value %s - will default to 1s: %v", timeout, err)
		th.waitTimeout = time.Duration(1 * time.Second) // default to 1s
	}
	if th.tileProcessingWorkers > 1 {
		th.tileProcessingJobQueue = make(chan *tileProcessingJob, tileProcessingJobQueueSize)
		th.tileProcessingDispatcher = newTileProcessingDispatcher(th.tileProcessingJobQueue, th.tileProcessingWorkers)
		logger.Infof("Wait for tile processing jobs (%d workers)", th.tileProcessingWorkers)
		th.tileProcessingDispatcher.run()
	}
	contentResultBufferSize := th.config.GetIntProperty("CONTENT_RESULT_BUFFER_SIZE", 0)
	if contentResultBufferSize > 0 {
		th.contentProcessedResBuffer = make(chan contentProcessingResult, contentResultBufferSize)
	}
	contentProcessingJobQueueSize := th.config.GetIntProperty("CONTENT_STORE_QUEUE_SIZE", 1)
	if contentProcessingJobQueueSize < 0 {
		logger.Errorf("Invalid contentProcessingJobQueueSize: %d", contentProcessingJobQueueSize)
		contentProcessingJobQueueSize = 1
	}
	if th.contentProcessingWorkers > 1 {
		logger.Infof("Wait for content store jobs (%d workers)", th.contentProcessingWorkers)
		th.contentProcessingJobQueue = make(chan *contentProcessingJob, contentProcessingJobQueueSize)
		th.contentProcessingDispatcher = newContentDispatcher(th.contentProcessingJobQueue, th.contentProcessingWorkers)
		th.contentProcessingDispatcher.run()
	}
	th.contentProcessingErrors = make(chan string, contentProcessingJobQueueSize)
}

func (th *TileRequestHandler) getProcessingQueuesStatus() (systemStatus, contentQueueStatus, tileQueueStatus QueueStatus) {
	contentQueueStatus = th.computeJobQueueStatus(len(th.contentProcessingJobQueue), cap(th.contentProcessingJobQueue))
	tileQueueStatus = th.computeJobQueueStatus(len(th.tileProcessingJobQueue), cap(th.tileProcessingJobQueue))
	if contentQueueStatus > tileQueueStatus {
		systemStatus = contentQueueStatus
	} else {
		systemStatus = tileQueueStatus
	}
	return
}

// SendMessage a TileRequestHandler implements an MessageNotifier interface
func (th *TileRequestHandler) SendMessage(message string, force bool) {
	select {
	case th.contentProcessingErrors <- message:
		return
	default:
	}
}

func (th *TileRequestHandler) checkForContentErrors() error {
	select {
	case errmessage := <-th.contentProcessingErrors:
		return fmt.Errorf(errmessage)
	default:
		return nil
	}
}

func (th *TileRequestHandler) startTileProcessingJob(job *tileProcessingJob) (*models.TemImage, error) {
	var resultErr error

	startTime := time.Now()
	resultChan := make(chan tileProcessingResult, 1)
	job.processor = decorateTileProcessor(th.tileProcessor,
		saveTileContent(th.tmpDataDir),
		concurrentTileProcessing(resultChan),
		errMonitoredTileProcessing(th.messageNotifier),
		loggedTileProcessing(startTime),
	)
	if th.tileProcessingWorkers > 1 {
		if err := th.enqueueTileProcessingJob(job); err != nil {
			return nil, err
		}
	} else {
		job.processor.process(job)
	}
	tileResult := <-resultChan
	if resultErr = tileResult.err; resultErr != nil {
		// if tile processing failed don't go further
		return tileResult.tileImage, resultErr
	}
	th.startStoreFileContent(&contentProcessingJob{
		tmpTileFile: job.tmpTileFile,
		acqID:       job.acqID,
		fileParams:  &job.tileParams.FileParams,
		tileImage:   tileResult.tileImage,
		execCtx:     job.execCtx,
		memPtr:      job.memPtr,
	})
	if contentProcessingErr := th.checkForContentErrors(); contentProcessingErr != nil {
		resultErr = contentProcessingErr
	}
	return tileResult.tileImage, resultErr
}

func (th *TileRequestHandler) enqueueTileProcessingJob(job *tileProcessingJob) error {
	// we do that in a select so that we know when the channel becomes blocked
	select {
	case th.tileProcessingJobQueue <- job:
		tileJobQueueStatus := th.computeJobQueueStatus(len(th.tileProcessingJobQueue), cap(th.tileProcessingJobQueue))
		logger.Debugf("Tile processing queue: %d items, status %s",
			len(th.tileProcessingJobQueue), tileJobQueueStatus.String())
		return nil
	default:
		logger.Infof("Tile processing queue (%d items) is full - tile queue status %s",
			len(th.tileProcessingJobQueue), Red.String())
		break
	}
	select {
	case th.tileProcessingJobQueue <- job:
		// if everything comes back before the timeout everything is still OK
		return nil
	case <-time.After(th.waitTimeout):
		// a timeout occurred - break the circuit
		return fmt.Errorf("Timeout waiting to enqueue the tile job for: %d:%s", job.acqID, job.tileParams.Name)
	}
}

func (th *TileRequestHandler) computeJobQueueStatus(size, capacity int) QueueStatus {
	if capacity == 0 {
		return Green
	}
	queueSizeCapRatio := float64(size) / float64(capacity)
	if queueSizeCapRatio < 0.7 {
		return Green
	} else if queueSizeCapRatio < 0.9 {
		return Yellow
	} else {
		return Red
	}
}

type tileProcessorImpl struct {
	service ImageCatcherService
}

func (p *tileProcessorImpl) process(job *tileProcessingJob) tileProcessingResult {
	acqMosaic, err := p.service.GetMosaic(job.acqID)
	if err != nil {
		return tileProcessingResult{nil, err}
	}
	job.acqMosaic = acqMosaic
	tileImage, err := p.service.StoreTile(job.acqMosaic, &job.tileParams)
	return tileProcessingResult{tileImage, err}
}

func (th *TileRequestHandler) startStoreFileContent(job *contentProcessingJob) error {
	var err error
	startTime := time.Now()
	job.processor = decorateContentProcessor(th.contentProcessor,
		updateTileMetadata(func(ti *models.TemImage) error {
			ti.SetAcquiredTimestamp(time.Now()) // update the acquired timestamp
			return th.imageCatcher.UpdateTile(ti)
		}),
		verifyContentProcessing(th.imageCatcher.VerifyAcquisitionFile),
		retryContentProcessing(defaultRetries),
		concurrentContentProcessing(th.contentProcessedResBuffer),
		errMonitoredContentProcessing(th, th.messageNotifier),
		loggedContentProcessing(startTime),
		removeTemporaryTileFile(),
		clearContentProcessing(),
	)
	if th.contentProcessingWorkers > 1 {
		if err = th.enqueueContentProcessingJob(job); err != nil {
			th.SendMessage(err.Error(), false)
		}
		return err
	}
	// the content processing is synchronous
	result := job.processor.process(job)
	return result.err
}

func (th *TileRequestHandler) enqueueContentProcessingJob(job *contentProcessingJob) error {
	// we do that in a select so that we know when the channel becomes blocked
	select {
	case th.contentProcessingJobQueue <- job:
		contentJobQueueStatus := th.computeJobQueueStatus(len(th.tileProcessingJobQueue), cap(th.tileProcessingJobQueue))
		logger.Debugf("Content processing queue: %d items, status %s",
			len(th.contentProcessingJobQueue), contentJobQueueStatus.String())
		return nil
	default:
		// if the channel becomes blocked we also increment
		logger.Infof("Content processing queue (%d items) is full - content queue status %s",
			len(th.contentProcessingJobQueue), Red.String())
		break
	}
	select {
	case th.contentProcessingJobQueue <- job:
		// if everything comes back before the timeout everything is still OK
		return nil
	case <-time.After(th.waitTimeout):
		// a timeout occurred - break the circuit
		return fmt.Errorf("Timeout waiting to enqueue the content processing job for: %d:%s", job.acqID, job.fileParams.Name)
	}
}

type contentProcessorImpl struct {
	service ImageCatcherService
}

func (p *contentProcessorImpl) process(job *contentProcessingJob) contentProcessingResult {
	fObj := job.getTileFile()
	err := p.service.StoreAcquisitionFile(job.getTileMosaic(), job.fileParams, fObj)
	return contentProcessingResult{fObj, err}
}
