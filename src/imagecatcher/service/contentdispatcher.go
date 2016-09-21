package service

import (
	"os"
	"time"
	"unsafe"

	"imagecatcher/logger"
	"imagecatcher/models"
	"imagecatcher/utils"
)

type contentProcessor interface {
	process(job *contentProcessingJob) contentProcessingResult
}

type contentProcessingJob struct {
	acqID       uint64
	fileParams  *FileParams
	tileImage   *models.TemImage
	processor   contentProcessor
	execCtx     ExecutionContext
	memPtr      unsafe.Pointer
	tmpTileFile string
}

func (j contentProcessingJob) getTileMosaic() *models.TemImageMosaic {
	return &j.tileImage.ImageMosaic
}

func (j contentProcessingJob) getTileFile() *models.FileObject {
	return &j.tileImage.TileFile
}

func (j *contentProcessingJob) clear() {
	utils.Free(j.memPtr)
	j.tileImage = nil
	j.fileParams = nil
	j.memPtr = nil
}

type contentProcessingResult struct {
	fObj *models.FileObject
	err  error
}

// contentDispatcher retrieves work from the jobQueue and then it
// it has a pool of job queues from which it picks one and sends the job to that
// queue
type contentDispatcher struct {
	workerPool chan chan *contentProcessingJob
	maxWorkers int
	jobQueue   chan *contentProcessingJob
	quit       chan struct{}
}

// contentWorker subscribes to a pool of workers by putting its own
// queue into the pool; when work gets put into its queue it picks it up
// and performs the corresponding processing
type contentWorker struct {
	jobQueue   chan *contentProcessingJob
	workerPool chan chan *contentProcessingJob
}

// newContentDispatcher creates a dispatcher for the master work queue and a number of workers
func newContentDispatcher(jobQueue chan *contentProcessingJob, maxWorkers int) *contentDispatcher {
	workerPool := make(chan chan *contentProcessingJob, maxWorkers)
	return &contentDispatcher{
		jobQueue:   jobQueue,
		maxWorkers: maxWorkers,
		workerPool: workerPool,
		quit:       make(chan struct{}),
	}
}

// run creates the workers starts them and then it starts the dispatcher
func (d *contentDispatcher) run() {
	for i := 0; i < d.maxWorkers; i++ {
		w := newContentWorker(d.workerPool)
		w.start()
	}
	go d.dispatch()
}

// dispatch is the method that looks up an available worker and
// then it transfers the job to that workers queue
func (d *contentDispatcher) dispatch() {
	for {
		select {
		case workerJobQueue := <-d.workerPool:
			job := <-d.jobQueue
			logger.Debugf("Dispatching content %d %s (%d): %v",
				job.acqID, job.fileParams.Name, len(job.fileParams.Content), time.Since(job.execCtx.StartTime))
			workerJobQueue <- job
		case <-d.quit:
			for workerJobQueue := range d.workerPool {
				close(workerJobQueue)
			}
			return
		}
	}
}

// dispatch is the method that looks up an available worker and
// then it transfers the job to that workers queue
func newContentWorker(workerPool chan chan *contentProcessingJob) *contentWorker {
	return &contentWorker{
		jobQueue:   make(chan *contentProcessingJob),
		workerPool: workerPool,
	}
}

// start the worker
func (w contentWorker) start() {
	go func() {
		for {
			// Ready for more work so tell the dispatcher where to send the work
			w.workerPool <- w.jobQueue
			// wait for work
			job, ok := <-w.jobQueue
			if ok {
				// Dispatcher has added a job to the queue so do the work
				job.processor.process(job)
			} else {
				// the channel was closed
				return
			}
		}
	}()
}

type contentProcessorFunc func(job *contentProcessingJob) contentProcessingResult

func (f contentProcessorFunc) process(job *contentProcessingJob) contentProcessingResult {
	return f(job)
}

type contentProcessDecorator func(p contentProcessor) contentProcessor

func loggedContentProcessing(localStartTime time.Time) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			logger.Debugf("Begin processing content %d:%s (%d): %v / %v",
				job.acqID, job.fileParams.Name, job.fileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			res := p.process(job)
			if res.err != nil {
				logger.Errorf("Error encountered while processing content %d:%s (%d): %v / %v",
					job.acqID, job.fileParams.Name, job.fileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			} else {
				logger.Infof("Successfully processed content %d:%s (%d): %v / %v",
					job.acqID, job.fileParams.Name, job.fileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			}
			return res
		})
	}
}

func concurrentContentProcessing(resultChan chan<- contentProcessingResult) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			select {
			case resultChan <- res:
			default:
			}
			return res
		})
	}
}

func errMonitoredContentProcessing(notifiers ...MessageNotifier) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			if res.err != nil {
				for _, notifier := range notifiers {
					notifier.SendMessage(res.err.Error(), false)
				}
			}
			return res
		})
	}
}

func clearContentProcessing() contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			job.clear()
			return res
		})
	}
}

func removeTemporaryTileFile() contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			if job.tmpTileFile != "" && res.err == nil {
				if _, ferr := os.Stat(job.tmpTileFile); os.IsNotExist(ferr) {
					logger.Infof("Temporary tile file %s not found", job.tmpTileFile)
				} else {
					ferr := os.Remove(job.tmpTileFile)
					if ferr != nil {
						logger.Errorf("Error deleting temporary file for %s: %v", job.tmpTileFile, ferr)
					}
					logger.Debugf("Removed temporary tile file %s (%v)", job.tmpTileFile, time.Since(job.execCtx.StartTime))
				}
				job.tmpTileFile = ""
			}
			return res
		})
	}
}

func retryContentProcessing(maxRetries int) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			if res.err == nil {
				return res
			}
			for r := 1; r < maxRetries; r++ {
				logger.Infof("Retry (%d) content processing  for %d:%s", r, job.acqID, job.fileParams.Name)
				res := p.process(job)
				if res.err == nil {
					return res
				}
			}
			logger.Errorf("Processing of %d:%s aborted after %d retries", job.acqID, job.fileParams.Name, maxRetries)
			return res
		})
	}
}

func verifyContentProcessing(verifyFunc func(*models.TemImageMosaic, *FileParams, *models.FileObject) error) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			if res.err == nil {
				localStartTime := time.Now()
				res.err = verifyFunc(job.getTileMosaic(), job.fileParams, job.getTileFile())
				logger.Debugf("Verified content content %d:%s (%d): %v / %v",
					job.acqID, job.fileParams.Name, job.fileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			} else {
				logger.Infof("Skipped verification for %d:%s because of a previous error", job.acqID, job.fileParams.Name)
			}
			return res
		})
	}
}

func updateTileMetadata(updateTileFunc func(*models.TemImage) error) contentProcessDecorator {
	return func(p contentProcessor) contentProcessor {
		return contentProcessorFunc(func(job *contentProcessingJob) contentProcessingResult {
			res := p.process(job)
			if res.err == nil {
				res.err = updateTileFunc(job.tileImage)
			}
			return res
		})
	}
}

func decorateContentProcessor(p contentProcessor, decorators ...contentProcessDecorator) contentProcessor {
	decorated := p
	for _, decorate := range decorators {
		decorated = decorate(decorated)
	}
	return decorated
}
