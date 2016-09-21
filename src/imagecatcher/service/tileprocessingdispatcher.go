package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"imagecatcher/logger"
	"imagecatcher/models"
)

type tileProcessor interface {
	process(job *tileProcessingJob) tileProcessingResult
}

type tileProcessingJob struct {
	acqID       uint64
	acqMosaic   *models.TemImageMosaic
	tileParams  TileParams
	processor   tileProcessor
	execCtx     ExecutionContext
	memPtr      unsafe.Pointer
	tmpTileFile string
}

func (tpj *tileProcessingJob) clear() {
	tpj.memPtr = nil
}

type tileProcessingResult struct {
	tileImage *models.TemImage
	err       error
}

// tileProcessingDispatcher retrieves work from the jobQueue and then it
// it has a pool of job queues from which it picks one and sends the job to that
// queue
type tileProcessingDispatcher struct {
	workerPool chan chan *tileProcessingJob
	maxWorkers int
	jobQueue   chan *tileProcessingJob
	quit       chan struct{}
}

// tileProcessingWorker subscribes to a pool of workers by putting its own
// queue into the pool; when work gets put into its queue it picks it up
// and performs the corresponding processing
type tileProcessingWorker struct {
	jobQueue   chan *tileProcessingJob
	workerPool chan chan *tileProcessingJob
}

// newTileProcessingDispatcher creates a dispatcher for the master work queue and a number of workers
func newTileProcessingDispatcher(jobQueue chan *tileProcessingJob, maxWorkers int) *tileProcessingDispatcher {
	workerPool := make(chan chan *tileProcessingJob, maxWorkers)
	return &tileProcessingDispatcher{
		jobQueue:   jobQueue,
		maxWorkers: maxWorkers,
		workerPool: workerPool,
		quit:       make(chan struct{}),
	}
}

// run creates the workers starts them and then it starts the dispatcher
func (d *tileProcessingDispatcher) run() {
	for i := 0; i < d.maxWorkers; i++ {
		w := newTileProcessingWorker(d.workerPool)
		w.start()
	}
	go d.dispatch()
}

// stop the dispatcher
func (d *tileProcessingDispatcher) stop() {
	go func() {
		var quit struct{}
		d.quit <- quit
	}()
}

// dispatch is the method that looks up an available worker and
// then it transfers the job to that workers queue
func (d *tileProcessingDispatcher) dispatch() {
	for {
		select {
		case workerJobQueue := <-d.workerPool:
			job := <-d.jobQueue
			execTime := time.Since(job.execCtx.StartTime)
			logger.Debugf("Dispatching tile %d %s (%d): %v",
				job.acqID, job.tileParams.Name, job.tileParams.ContentLen, execTime)
			workerJobQueue <- job
		case <-d.quit:
			for workerJobQueue := range d.workerPool {
				close(workerJobQueue)
			}
			return
		}
	}
}

// newTileProcessingWorker creates a worker and gives it a pool to "subscribe"
func newTileProcessingWorker(workerPool chan chan *tileProcessingJob) *tileProcessingWorker {
	return &tileProcessingWorker{
		jobQueue:   make(chan *tileProcessingJob),
		workerPool: workerPool,
	}
}

// start the worker
func (w tileProcessingWorker) start() {
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

type tileProcessorFunc func(job *tileProcessingJob) tileProcessingResult

type tileProcessDecorator func(p tileProcessor) tileProcessor

func (f tileProcessorFunc) process(job *tileProcessingJob) tileProcessingResult {
	return f(job)
}

func loggedTileProcessing(localStartTime time.Time) tileProcessDecorator {
	return func(p tileProcessor) tileProcessor {
		return tileProcessorFunc(func(job *tileProcessingJob) tileProcessingResult {
			logger.Debugf("Begin processing tile %d:%s (%d): %v / %v",
				job.acqID, job.tileParams.Name, job.tileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			res := p.process(job)
			if res.err != nil {
				logger.Infof("Error encountered while processing tile %d:%s (%d): %v / %v",
					job.acqID, job.tileParams.Name, job.tileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			} else {
				logger.Infof("Successfully processed tile %d:%s (%d): %v / %v",
					job.acqID, job.tileParams.Name, job.tileParams.ContentLen, time.Since(localStartTime), time.Since(job.execCtx.StartTime))
			}
			return res
		})
	}
}

func concurrentTileProcessing(resultChan chan<- tileProcessingResult) tileProcessDecorator {
	return func(p tileProcessor) tileProcessor {
		return tileProcessorFunc(func(job *tileProcessingJob) tileProcessingResult {
			res := p.process(job)
			select {
			case resultChan <- res:
			default:
			}
			return res
		})
	}
}

func errMonitoredTileProcessing(notifier MessageNotifier) tileProcessDecorator {
	return func(p tileProcessor) tileProcessor {
		return tileProcessorFunc(func(job *tileProcessingJob) tileProcessingResult {
			res := p.process(job)
			if res.err == nil && res.tileImage == nil {
				res.err = fmt.Errorf("Internal Error: Invalid tile image set in the result")
			}
			if res.err != nil {
				notifier.SendMessage(res.err.Error(), false)
			}
			return res
		})
	}
}

func saveTileContent(dataDir string) tileProcessDecorator {
	return func(p tileProcessor) tileProcessor {
		return tileProcessorFunc(func(job *tileProcessingJob) tileProcessingResult {
			var err error
			fParams := &job.tileParams.FileParams
			if job.tmpTileFile, err = createTmpDataFile(dataDir, job.acqID, fParams, job.execCtx); err != nil {
				return tileProcessingResult{nil, fmt.Errorf("Error creating the temporary file for %d:%s - %v", job.acqID, fParams.Name, err)}
			}
			return p.process(job)
		})
	}
}

func createTmpDataFile(dataDir string, acqID uint64, f *FileParams, execCtx ExecutionContext) (string, error) {
	startTime := time.Now()

	fullFilename := filepath.Join(dataDir, fmt.Sprintf("%d", acqID), f.Name)

	parentDir := filepath.Dir(fullFilename)
	err := os.MkdirAll(parentDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("Error creating directory %s for writing temporary file to disc: %v", parentDir, err)
	}
	err = ioutil.WriteFile(fullFilename, f.Content, os.ModePerm)
	if err != nil {
		return fullFilename, fmt.Errorf("Error writing temporary file %s to disc: %v", fullFilename, err)
	}

	logger.Debugf("Created temporary tile file %s (%d): %v / %v", fullFilename, f.ContentLen, time.Since(startTime), time.Since(execCtx.StartTime))
	return fullFilename, err
}

func decorateTileProcessor(p tileProcessor, decorators ...tileProcessDecorator) tileProcessor {
	decorated := p
	for _, decorate := range decorators {
		decorated = decorate(decorated)
	}
	return decorated
}
