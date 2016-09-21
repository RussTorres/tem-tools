package daotest

import (
	"fmt"
	"time"

	"imagecatcher/dao"
	"imagecatcher/models"
	"imagecatcher/utils"
)

var (
	testTileSpec = `
		[Inclusion Section Numbers]
		Region 0 = "100"
		Region 1 = "101"
		Region 2 = "102"
	`
	testTileROIs = `
		Col,Row,Inclusion,Excluded?,Included?,# Inclusions,Section number
		0,0,-1,0,0,0,
		1,1,-1,0,0,0,
		1,2,0,0,1,1,100
		1,2,1,0,1,1,101
		1,2,2,0,1,1,102
		1,3,0,0,1,1,100
		1,4,0,0,1,1,100
		1,5,0,0,1,1,100
		2,0,1,0,1,1,101
		2,1,1,0,1,1,101
		2,2,1,0,1,1,101
		2,3,1,0,1,1,101
		3,0,2,0,1,1,102
		3,1,2,0,1,1,102
		3,2,2,0,1,1,102
		3,3,2,0,1,1,102
	`
)

func createTestMosaic(sampleName string, acqID uint64, session dao.DbSession) (*models.Acquisition, *models.TemImageMosaic, error) {
	acq := &models.Acquisition{
		AcqUID:          acqID,
		SampleName:      sampleName,
		TemIPAddress:    "10.101.50.60",
		Acquired:        time.Now().Add(time.Duration(acqID) * time.Minute),
		Inserted:        time.Now(),
		NumberOfCameras: 1,
	}
	if err := dao.CreateAcqLog(acq, session); err != nil {
		return acq, nil, fmt.Errorf("Error creating the acquisition log: %v", err)
	}
	imageMosaic, err := dao.CreateImageMosaic(acq, session)
	return acq, imageMosaic, err
}

func createTestTiles(imageMosaic *models.TemImageMosaic, tileState string, session dao.DbSession) (nTiles, nRois int, err error) {
	regionToAcqROIMap, err := utils.ParseTileSpec([]byte(testTileSpec))
	if err != nil {
		return nTiles, nRois, fmt.Errorf("Error parsing tile spec: %v", err)
	}
	tiles, err := utils.ParseROITiles([]byte(testTileROIs))
	if err != nil {
		return nTiles, nRois, fmt.Errorf("Error parsing tile rois: %v", err)
	}
	for _, acqRoi := range regionToAcqROIMap {
		if _, err = dao.CreateROI(imageMosaic, acqRoi, session); err != nil {
			return nTiles, nRois, fmt.Errorf("Error creating ROI: %v", err)
		}
		nRois++
	}
	cameraConfigurations, err := dao.RetrieveCameraConfigurationsByTemcaID(imageMosaic.Temca.TemcaID, session)
	if err != nil {
		return nTiles, nRois, fmt.Errorf("No camera configuration found for temca %d: %v", imageMosaic.Temca.TemcaID, err)
	}
	camera := 1
	cameraConfiguration := cameraConfigurations[camera]
	// for each tile create a drift frame and a stable frame
	for _, ti := range tiles {
		// Create the drift frame
		_, err := dao.CreateTemImage(imageMosaic, ti.Col, ti.Row, 1, session)
		if err != nil {
			return nTiles, nRois, fmt.Errorf("Error creating drift tile image: %v", err)
		}
		// Create TemImage for the stable frame
		temImage, err := dao.CreateTemImage(imageMosaic, ti.Col, ti.Row, -1, session)
		if err != nil {
			return nTiles, nRois, fmt.Errorf("Error creating stable tile image: %v", err)
		}
		// update camera configuration
		temImage.Configuration = &cameraConfiguration
		dao.UpdateTemImageCamera(temImage, session)
		// update tile file fields
		tileName := fmt.Sprintf("%d_%d_%d.tif", ti.Col, ti.Row, camera)
		temImage.TileFile.Path = tileName
		temImage.TileFile.JfsPath = tileName
		temImage.TileFile.JfsKey = tileName
		if err = dao.UpdateFileObj(&temImage.TileFile, session); err != nil {
			return nTiles, nRois, fmt.Errorf("Error updating tile file parameters: %v", err)
		}
		// Create ROI Tile
		if _, err = dao.CreateTileROI(imageMosaic, temImage.ImageID, regionToAcqROIMap[ti.Roi.RegionName], tileState, session); err != nil {
			break
		}
		nTiles++
	}
	return nTiles, nRois, nil
}
