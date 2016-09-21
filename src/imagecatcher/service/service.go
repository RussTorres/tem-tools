package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"imagecatcher/config"
	"imagecatcher/diagnostic"
	"imagecatcher/logger"
	"imagecatcher/models"
)

// ServerHandler interface for dealing with TCP request
type ServerHandler interface {
	Serve() error
}

// Initializer objects that could be initilized using a configuration
type Initializer interface {
	Initialize(config config.Config) error
}

// AcqDataWriter handles the creation of new acquisition specific entities
type AcqDataWriter interface {
	CreateAcquisition(acqLog *FileParams) (*models.Acquisition, error)
	CreateAncillaryFile(acqMosaic *models.TemImageMosaic, f *FileParams) (*models.FileObject, error)
	CreateROIs(acqMosaic *models.TemImageMosaic, roiFiles *ROIFiles) error
	EndAcquisition(acqID uint64) error
	StoreTile(acqMosaic *models.TemImageMosaic, tileParams *TileParams) (*models.TemImage, error)
	StoreAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) error
	UpdateTile(tileImage *models.TemImage) error
}

// AcqDataReader handles the retrieval of acquisition entities
type AcqDataReader interface {
	GetAcquisitions(acqFilter *models.AcquisitionFilter) ([]*models.Acquisition, error)
	GetMosaic(acqID uint64) (*models.TemImageMosaic, error)
	GetTile(tileID int64) (*models.TemImageROI, error)
	GetTiles(tileFilter *models.TileFilter) ([]*models.TemImageROI, error)
	RetrieveAcquisitionFile(acqMosaic *models.TemImageMosaic, path string) ([]byte, error)
	VerifyAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) error
}

// ProjectDataReader retrieves acquisition projects info
type ProjectDataReader interface {
	GetProjects(filter *models.ProjectFilter) ([]*models.Project, error)
}

// ImageCatcherService processes acquistion data
type ImageCatcherService interface {
	AcqDataWriter
	AcqDataReader
	ProjectDataReader
	diagnostic.Pingable
}

// FileParams file specific parameters
type FileParams struct {
	Name              string
	Content, Checksum []byte
	ContentLen        int
}

func (f *FileParams) isEmpty() bool {
	return len(f.Content) == 0
}

func (f *FileParams) reset() {
	f.Name = ""
	f.Content = nil
	f.Checksum = nil
	f.ContentLen = 0
}

// TileParams tile specific parameters
type TileParams struct {
	FileParams
	Col, Row, Camera, Frame int
}

// ROIFiles files used for creating tile to ROIs mapping
type ROIFiles struct {
	RoiTiles FileParams
	RoiSpec  FileParams
}

// ExecutionContext used for capturing the state during the processing of a single request.
// For now it only captures when the processing starts.
type ExecutionContext struct {
	StartTime time.Time
}

var (
	tileFileNameRegexp = regexp.MustCompile("col(\\d+)_row(\\d+)_cam(\\d+)\\.tif")
	dumpFileNameRegexp = regexp.MustCompile("Dump_cam(\\d)_frame(\\d+)_col(\\d+)_row(\\d+)\\.tif")
)

// ErrInvalidTileFileName - name of the tile filename is invalid
var ErrInvalidTileFileName = errors.New("Invalid tile file name")

// String representation of TileParams
func (t TileParams) String() string {
	return fmt.Sprintf("{col: %d, row: %d, camera: %d, frame: %d, name: %s}",
		t.Col, t.Row, t.Camera, t.Frame, t.Name)
}

// ExtractTileParamsFromURL extracts the tile paramenters from a file name.
func (t *TileParams) ExtractTileParamsFromURL(url string) error {
	var err, resErr error

	urlcomponents := strings.Split(url, "/")
	filename := urlcomponents[len(urlcomponents)-1]
	if tileFileNameRegexp.MatchString(filename) {
		t.Name = filename
		matches := tileFileNameRegexp.FindStringSubmatch(filename)
		if t.Col, err = strconv.Atoi(matches[1]); err != nil {
			logger.Errorf("Error parsing the tile column from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		if t.Row, err = strconv.Atoi(matches[2]); err != nil {
			logger.Errorf("Error parsing the tile row from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		if t.Camera, err = strconv.Atoi(matches[3]); err != nil {
			logger.Errorf("Error parsing the tile camera from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		t.Frame = -1
	} else if dumpFileNameRegexp.MatchString(filename) {
		t.Name = filename
		matches := dumpFileNameRegexp.FindStringSubmatch(filename)
		if t.Col, err = strconv.Atoi(matches[3]); err != nil {
			logger.Errorf("Error parsing the tile column from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		if t.Row, err = strconv.Atoi(matches[4]); err != nil {
			logger.Errorf("Error parsing the tile row from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		if t.Camera, err = strconv.Atoi(matches[1]); err != nil {
			logger.Errorf("Error parsing the tile camera from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
		if t.Frame, err = strconv.Atoi(matches[2]); err != nil {
			logger.Errorf("Error parsing the image frame from %s: %v", url, err)
			resErr = ErrInvalidTileFileName
		}
	} else {
		logger.Errorf("Tile file name does not match any of the recognizable patterns: %s", url)
		resErr = ErrInvalidTileFileName
	}
	return resErr
}

// UpdateTileName - if the tile parameters are set it creates the corresponding file name.
func (t *TileParams) UpdateTileName() {
	if t.Frame < 0 {
		t.Name = fmt.Sprintf("col%04d_row%04d_cam%d.tif", t.Col, t.Row, t.Camera)
	} else {
		t.Name = fmt.Sprintf("Dump_cam%d_frame%06d_col%04d_row%04d.tif", t.Camera, t.Frame, t.Col, t.Row)
	}
}
