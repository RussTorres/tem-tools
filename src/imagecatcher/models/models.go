package models

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"imagecatcher/logger"
)

const (
	// TileCreateState - value for tile created state
	TileCreateState = "CREATE"
	// TileReadyState - value for tile ready state
	TileReadyState = "READY"
	// TileInProgressState - value for tile in progress
	TileInProgressState = "IN_PROGRESS"
)

// FrameTypeFilter defines whether the frame is stable, drift or any
type FrameTypeFilter int

const (
	// Stable - stable frames only
	Stable FrameTypeFilter = iota
	// Drift - only drift frames
	Drift
	// All - all frames - both stable and drift
	All
)

// Set the frame type value
func (ft *FrameTypeFilter) Set(value string) (err error) {
	switch strings.ToLower(value) {
	case "all":
		*ft = All
	case "drift":
		*ft = Drift
	case "stable", "":
		*ft = Stable
	default:
		logger.Infof("Invalid frame type: %s - will consider stable instead", value)
		*ft = Stable
	}
	return
}

// TemImage - entity for a tile
type TemImage struct {
	ImageID           int64
	Col, Row, Frame   int
	ImageMosaic       TemImageMosaic
	TileFile          FileObject
	Configuration     *TemCameraConfiguration
	AcquiredTimestamp *time.Time
}

// String - string representation of a tile
func (ti TemImage) String() string {
	camera := -1
	if ti.Configuration != nil {
		camera = ti.Configuration.Camera
	}
	return fmt.Sprintf("{id: %d, col: %d, row: %d, camera: %d, frame: %d}",
		ti.ImageID, ti.Col, ti.Row, camera, ti.Frame)
}

// SetAcquiredTimestamp - update the acquired timestamp
func (ti *TemImage) SetAcquiredTimestamp(t time.Time) {
	ti.AcquiredTimestamp = new(time.Time)
	*ti.AcquiredTimestamp = t
}

// AcqROI represents a region of interest at the acquisition time
type AcqROI struct {
	MosaicRoiID    int64
	AcqRoiID       int64
	RegionName     string
	NominalSection int64
	SectionName    string
}

// String - AcqROI string representation
func (r AcqROI) String() string {
	return fmt.Sprintf("{mosaic_roi_id: %d, acq_roi_id: %d, region: %s, section: %s, nominalSection: %d}",
		r.MosaicRoiID, r.AcqRoiID, r.RegionName, r.SectionName, r.NominalSection)
}

// TemImageROI represents a TemImage with a region of interest
type TemImageROI struct {
	TemImage
	RoiImageID int64
	Roi        AcqROI
	State      string
}

// String - string represenation of a tile
func (ti TemImageROI) String() string {
	camera := -1
	if ti.Configuration != nil {
		camera = ti.Configuration.Camera
	}
	return fmt.Sprintf("{id: %d, col: %d, row: %d, camera: %d, frame: %d, region: %s, section: %s}",
		ti.ImageID, ti.Col, ti.Row, camera, ti.Frame, ti.Roi.RegionName, ti.Roi.SectionName)
}

// TemImageMosaic - image mosaic entity
type TemImageMosaic struct {
	Acquisition
	Temca            Temca
	NumCols, NumRows int
}

// TemCameraConfiguration - entity for a TEM camera configuration
type TemCameraConfiguration struct {
	ConfigurationID, CameraID              int64
	Camera, CameraArrayCol, CameraArrayRow int
	Width, Height                          int
	TransformationRef                      string
	MaskURL                                string
	TemcaID                                int64
}

// String - string representation of a TemCameraConfiguration
func (tcc TemCameraConfiguration) String() string {
	return fmt.Sprintf("{id: %d, temca: %d, camera: %d}", tcc.ConfigurationID, tcc.TemcaID, tcc.Camera)
}

// Temca - entity for a Temca
type Temca struct {
	TemcaID, TemID, TemScintillatorID, TemcaTypeID int64
}

// FileObject - file object entity
type FileObject struct {
	FileObjectID, FileFSID int64
	Path, JfsPath, JfsKey  string
	LocationURL            string
	Checksum               []byte
	ChecksumTimestamp      *time.Time
}

// SetChecksumTimestamp - update the checksum timestamp
func (f *FileObject) SetChecksumTimestamp(t time.Time) {
	f.ChecksumTimestamp = new(time.Time)
	*f.ChecksumTimestamp = t
}

// UpdateJFSAndPathParams - updates JFS and path attributes
func (f *FileObject) UpdateJFSAndPathParams(filePath string, jfsParams map[string]interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error encountered while updating JFS parameters: %v", r)
		}
	}()
	f.Path = filePath
	f.JfsPath = jfsParams["jfsPath"].(string)
	f.JfsKey = jfsParams["scalityKey"].(string)
	if jfsParams["locationUrl"] != nil {
		f.LocationURL = jfsParams["locationUrl"].(string)
	}
	if jfsParams["checksum"] != nil {
		checksum := jfsParams["checksum"].(string)
		logger.Debugf("Checksum: %s", checksum)
		if f.Checksum, err = hex.DecodeString(checksum); err != nil {
			decodeErr := fmt.Errorf("Error decoding the checksum %s: %v", checksum, err)
			logger.Error(decodeErr)
			return decodeErr
		}
	}
	f.SetChecksumTimestamp(time.Now())
	return
}

// Acquisition - acquisition entity
type Acquisition struct {
	IniFileID, ImageMosaicID     int64
	Acquired, Inserted           time.Time
	Completed                    *time.Time
	AcqUID                       uint64
	NumberOfCameras              int
	XSmallStepPix, YSmallStepPix int
	XBigStepPix, YBigStepPix     int
	NXSteps, NYSteps             int
	NTargetCols, NTargetRows     int
	XCenter, YCenter, YTem       float64
	PixPerUm                     float64
	Magnification                int
	TemIPAddress                 string
	URL                          string
	SampleName                   string
	ProjectName                  string
	ProjectOwner                 string
	StackName                    string
	MosaicType                   string
	RoiTilesFile, RoiSpecFile    string
	MicroscopistName, Notes      string
	IniContent                   string
}

// String - string representation of an acquisition
func (a Acquisition) String() string {
	return fmt.Sprintf("{uid: %d, iniId: %d, mosaicId: %d}", a.AcqUID, a.IniFileID, a.ImageMosaicID)
}

// SetCompletedTimestamp - update the completed timestamp
func (a *Acquisition) SetCompletedTimestamp(t time.Time) {
	a.Completed = new(time.Time)
	*a.Completed = t
}

// IsCompleted returns true if the acquisition completed, i.e. a.Completed != nil
func (a Acquisition) IsCompleted() bool {
	return a.Completed != nil
}

// AcquisitionFilter acquisition filter parameters
type AcquisitionFilter struct {
	Acquisition
	RequiredStateForAllTiles       string
	RequiredStateForAtLeastOneTile string
	AcquiredInterval               TimeInterval
	Pagination                     Page
}

// Project - project entity
type Project struct {
	ProjectName  string
	ProjectOwner string
}

// ProjectFilter project filter parameters
type ProjectFilter struct {
	Acquisition
	DataAcquiredInterval TimeInterval
	Pagination           Page
}

// TileSpec - a tile representation that will served to the clients that need to be served tiles
type TileSpec struct {
	Acquisition
	TileImageID         int64
	RoiImageID          int64
	TileSpecID          string
	NominalSection      int64
	Col, Row, Frame     int
	Width, Height       int
	JFSKey, JFSPath     string
	State               string
	PrevAcqCount        int
	ImageURL, MaskURL   string
	TransformationRefID string
	CameraConfig        TemCameraConfiguration
}

// SetTileImageURL set tile image url
func (ts *TileSpec) SetTileImageURL(host string) {
	imageURL := ts.ImageURL + "?fName=" + ts.JFSPath
	ts.ImageURL = imageURL
}

// SetMaskURL updates tile mask url based on the mask root set in current cameraConfig
func (ts *TileSpec) SetMaskURL() {
	ts.MaskURL = ts.CameraConfig.MaskURL
}

// SetTransformationRefID updates transformation Ref ID based on the pattern defined in current cameraConfig
func (ts *TileSpec) SetTransformationRefID() {
	ts.TransformationRefID = ts.CameraConfig.TransformationRef
}

// TileFilter tile filter parameters
type TileFilter struct {
	TileSpec
	FrameType            FrameTypeFilter
	IncludeNotPersisted  bool
	TileAcquiredInterval TimeInterval
	Pagination           Page
}

// IncludeOnlyPersisted specifies whether to only include persisted tiles
func (tf TileFilter) IncludeOnlyPersisted() bool {
	return !tf.IncludeNotPersisted
}

// Calibration represents a calibration
type Calibration struct {
	ID          int64
	Name        string
	Generated   time.Time
	JSONContent string
	Notes       string
	TemcaID     int64
	Camera      int
}

// Page contains pagination data
type Page struct {
	StartRecordIndex int64
	NRecords         int32
}

// TimeInterval defines the lower and uppper bounds of a time interval
// The convention is that the lower bound is closed (included) and the upper bound is opened (not included)
type TimeInterval struct {
	From, To *time.Time
}
