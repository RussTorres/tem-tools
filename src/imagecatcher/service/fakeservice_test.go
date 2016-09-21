package service

import (
	"fmt"

	"imagecatcher/config"
	"imagecatcher/models"
	"imagecatcher/utils"
)

var defaultTestingConfig = config.Config{
	"TILE_PROCESSING_WORKERS":    10,
	"TILE_PROCESSING_QUEUE_SIZE": 10,
	"CONTENT_STORE_WORKERS":      10,
	"CONTENT_STORE_QUEUE_SIZE":   10,
}

type fakeImageCatcherService struct {
	ImageCatcherService
	tileData, tileContent *localStorage
}

func (s fakeImageCatcherService) StoreTile(acqMosaic *models.TemImageMosaic, tileParams *TileParams) (i *models.TemImage, err error) {
	defer func() {
		if r := recover(); r != nil {
			i = nil
			err = fmt.Errorf("%v", r)
		}
	}()
	s.tileData.add(*tileParams)
	return &models.TemImage{
		Col:           tileParams.Col,
		Row:           tileParams.Row,
		Configuration: &models.TemCameraConfiguration{Camera: tileParams.Camera},
		Frame:         tileParams.Frame,
	}, nil
}

func (s fakeImageCatcherService) UpdateTile(tileImage *models.TemImage) error {
	return nil
}

func (s fakeImageCatcherService) CreateAcquisition(acqLog *FileParams) (*models.Acquisition, error) {
	acq, err := utils.ParseAcqIniLog(acqLog.Content)
	if err != nil {
		return acq, err
	}
	acq.AcqUID, err = utils.GetAcqUIDFromURL(acqLog.Name)
	return acq, err
}

func (s fakeImageCatcherService) CreateROIs(acqMosaic *models.TemImageMosaic, roiFiles *ROIFiles) error {
	// process ROI Spec
	if _, err := utils.ParseTileSpec(roiFiles.RoiSpec.Content); err != nil {
		return err
	}
	// process ROI Tiles
	if _, err := utils.ParseROITiles(roiFiles.RoiTiles.Content); err != nil {
		return err
	}
	return nil
}

func (s fakeImageCatcherService) CreateAncillaryFile(acqMosaic *models.TemImageMosaic, f *FileParams) (*models.FileObject, error) {
	fObj := &models.FileObject{}
	var err error
	fileType := extractAncillaryFileType(f.Name)
	switch fileType {
	case roiSpecFileType:
		// process ROI Spec
		if _, err = utils.ParseTileSpec(f.Content); err != nil {
			return fObj, err
		}
		return fObj, err
	case roiTilesFileType:
		// process ROI Tiles
		if _, err = utils.ParseROITiles(f.Content); err != nil {
			return fObj, err
		}
		return fObj, err
	default:
		return fObj, err
	}
}

func (s fakeImageCatcherService) EndAcquisition(acqID uint64) error {
	return nil
}

func (s fakeImageCatcherService) StoreAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) (err error) {
	err = nil
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	s.tileContent.add(*f)
	return
}

func (s fakeImageCatcherService) VerifyAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) error {
	return nil // dummy implementation that never returns errors
}

func (s fakeImageCatcherService) GetMosaic(acqID uint64) (*models.TemImageMosaic, error) {
	return &models.TemImageMosaic{
		Acquisition: models.Acquisition{AcqUID: acqID},
	}, nil
}

func (s fakeImageCatcherService) GetAcquisitions(acqFilter *models.AcquisitionFilter) ([]*models.Acquisition, error) {
	return []*models.Acquisition{}, nil
}

func (s *fakeImageCatcherService) GetProjects(filter *models.ProjectFilter) ([]*models.Project, error) {
	projects := []*models.Project{
		&models.Project{
			ProjectName:  "p1.1",
			ProjectOwner: "o1",
		},
		&models.Project{
			ProjectName:  "p1.2",
			ProjectOwner: "o1",
		},
		&models.Project{
			ProjectName:  "p2.1",
			ProjectOwner: "o2",
		},
	}
	if filter.ProjectOwner == "" && filter.ProjectName == "" {
		return projects, nil
	}
	var res []*models.Project
	for _, p := range projects {
		if filter.ProjectName != "" && filter.ProjectName != p.ProjectName {
			continue
		}
		if filter.ProjectOwner != "" && filter.ProjectOwner != p.ProjectOwner {
			continue
		}
		res = append(res, p)
	}
	return res, nil
}

type fakeErrorSender struct {
	messageStorage *localStorage
}

func (n *fakeErrorSender) SendMessage(message string, force bool) {
	n.messageStorage.add(message)
}

func createFakeErrorNotifier() *fakeErrorSender {
	return &fakeErrorSender{
		messageStorage: createLocalStorage(),
	}
}
