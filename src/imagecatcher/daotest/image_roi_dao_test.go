package daotest

import (
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"imagecatcher/dao"
	"imagecatcher/models"
)

func init() {
	InitTestDB()
}

func TestTileROICountByAcqIDAndSection(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	acq, imageMosaic, err := createTestMosaic("Test sample", 100, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	if err = verifyTilesCountStatusByAcqAndSection(acq, session); err != nil {
		t.Error(err)
	}
}

func TestGetTileROIsByAcqIDSectionAndState(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	sampleName := "Test sample"
	var acqID uint64 = 100
	acq, imageMosaic, err := createTestMosaic(sampleName, acqID, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	var tileFilter models.TileFilter
	tileFilter.AcqUID = acq.AcqUID
	tileFilter.IncludeNotPersisted = true
	tileFilter.FrameType = models.Stable
	for tileFilter.NominalSection = 100; tileFilter.NominalSection < 103; tileFilter.NominalSection++ {
		var nUpdates int64
		tileFilter.State = "READY"
		tileFilter.Pagination.NRecords = 1
		readyTiles, err := dao.UpdateTilesStatus(&tileFilter, "TMP", session)
		if err != nil {
			t.Error("Unexpected error retrieving 1 READY tile for section ", tileFilter.NominalSection, err)
		}
		if len(readyTiles) != 1 {
			t.Error("Expected to retrieve 1 READY tile for section ", tileFilter.NominalSection, " but it got ", len(readyTiles))
		}
		if nUpdates, err = dao.UpdateTemImageROIState(readyTiles, "INPROGRESS", session); err != nil {
			t.Error("Unexpected error while updating the state to INPROGRESS for ", readyTiles)
		}
		tileFilter.State = "INPROGRESS"
		tileFilter.Pagination.NRecords = 0
		inProgressTiles, err := dao.UpdateTilesStatus(&tileFilter, "COMPLETE", session)
		if err != nil {
			t.Error("Unexpected error retrieving INPROGRESS tiles for section ", tileFilter.NominalSection, err)
		}
		if len(inProgressTiles) != len(readyTiles) || int(nUpdates) != len(readyTiles) {
			t.Error("Expected to retrieve ", len(readyTiles), " INPROGRESS tiles but it got ", len(inProgressTiles))
		}
		if inProgressTiles[0].RoiImageID != readyTiles[0].RoiImageID {
			t.Error("Expected to INPROGRESS tile to be ", readyTiles[0].RoiImageID, " but it got ", inProgressTiles[0].RoiImageID)
		}
	}
}

func TestTileROICountByAcqID(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	acq, imageMosaic, err := createTestMosaic("Test sample", 100, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	if err = verifyTilesCountStatusByAcq(acq, session); err != nil {
		t.Error(err)
	}
}

func TestGetTileROIsByAcqIDAndState(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	sampleName := "Test sample"
	var acqID uint64 = 100
	acq, imageMosaic, err := createTestMosaic(sampleName, acqID, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	var tileFilter models.TileFilter
	tileFilter.AcqUID = acq.AcqUID
	tileFilter.NominalSection = -1
	tileFilter.State = "READY"
	tileFilter.IncludeNotPersisted = false
	tileFilter.FrameType = models.Stable
	readyTiles, err := dao.UpdateTilesStatus(&tileFilter, "TMP", session)
	if err != nil {
		t.Error("Unexpected error retrieving READY tiles for acquisition ", acq.AcqUID, err)
	}
	if len(readyTiles) == 0 {
		t.Error("Expected the number of READY tiles to be > 0 for acquisition", acq.AcqUID)
	}
	var nUpdates int64
	if nUpdates, err = dao.UpdateTemImageROIState(readyTiles, "INPROGRESS", session); err != nil {
		t.Error("Unexpected error while updating the state to INPROGRESS for ", readyTiles)
	}
	tileFilter.State = "INPROGRESS"
	inProgressTiles, err := dao.UpdateTilesStatus(&tileFilter, "INPROGRESS", session)
	if err != nil {
		t.Error("Unexpected error retrieving remaining INPROGRESS tiles for acquisition ", acq.AcqUID, err)
	}
	tileFilter.State = "READY"
	remainingReadyTiles, err := dao.UpdateTilesStatus(&tileFilter, "TMP", session)
	if err != nil {
		t.Error("Unexpected error retrieving remaining READY tiles for acquisition ", acq.AcqUID, err)
	}
	if len(inProgressTiles) != len(readyTiles) || int(nUpdates) != len(readyTiles) {
		t.Error("Expected to retrieve ", len(readyTiles), " INPROGRESS tiles but it got ", len(inProgressTiles))
	}
	if len(remainingReadyTiles) != 0 {
		t.Error("Expected to not find any READY tiles for acquisition ", acq.AcqUID, " but it found ", len(remainingReadyTiles))
	}
}

func TestUpdateStateByTileParamsOnly(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	sampleName := "Test sample"
	var acqID uint64 = 100
	acq, imageMosaic, err := createTestMosaic(sampleName, acqID, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	var tileFilter models.TileFilter
	tileFilter.AcqUID = acq.AcqUID
	tileFilter.NominalSection = -1
	tileFilter.State = "READY"
	readyTiles, err := dao.UpdateTilesStatus(&tileFilter, "TMP", session)
	if err != nil {
		t.Error("Unexpected error retrieving READY tiles for acquisition ", acq.AcqUID, err)
	}
	if len(readyTiles) == 0 {
		t.Error("Expected the number of READY tiles to be > 0 for acquisition", acq.AcqUID)
	}
	for _, t := range readyTiles {
		t.TileImageID = 0
		t.RoiImageID = 0
	}
	var nUpdates int64
	if nUpdates, err = dao.UpdateTemImageROIState(readyTiles, "INPROGRESS", session); err != nil {
		t.Error("Unexpected error while updating the state to INPROGRESS for ", readyTiles)
	}
	tileFilter.State = "INPROGRESS"
	inProgressTiles, err := dao.UpdateTilesStatus(&tileFilter, "COMPLETE", session)
	if err != nil {
		t.Error("Unexpected error retrieving remaining INPROGRESS tiles for acquisition ", acq.AcqUID, err)
	}
	tileFilter.State = "READY"
	remainingReadyTiles, err := dao.UpdateTilesStatus(&tileFilter, "INPROGRESS", session)
	if err != nil {
		t.Error("Unexpected error retrieving remaining READY tiles for acquisition ", acq.AcqUID, err)
	}
	if len(inProgressTiles) != len(readyTiles) || int(nUpdates) != len(readyTiles) {
		t.Error("Expected to retrieve ", len(readyTiles), " INPROGRESS tiles but it got ", len(inProgressTiles))
	}
	if len(remainingReadyTiles) != 0 {
		t.Error("Expected to not find any READY tiles for acquisition ", acq.AcqUID, " but it found ", len(remainingReadyTiles))
	}
}

func TestTileROICountBySample(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	nacqs := 3
	acqs := make([]*models.Acquisition, nacqs)
	sampleName := "Test sample"
	for i := 0; i < nacqs; i++ {
		acq, imageMosaic, err := createTestMosaic(sampleName, 100+uint64(i), session)
		if err != nil {
			log.Panic(err)
		}
		if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
			log.Panic(err)
		}
		acqs[i] = acq
	}
	if err = verifyTilesCountStatusBySample(sampleName, acqs, session); err != nil {
		t.Error(err)
	}
	testSections := []int64{100, 101, 102}
	for i, acq := range acqs {
		for _, testSection := range testSections {
			c, err := dao.CountPrevAcqContainingSection(sampleName, testSection, acq.Acquired.Add(-1*time.Second), session)
			if err != nil {
				t.Error("Unexpected error while retrieving previous acquisitions for section ", testSection, ":", err)
			} else if c != i {
				t.Error("Expected exactly ", i, " previous acquisitions for section ", testSection, " but got ", c,
					"acquisitions before ", acq.AcqUID, " acquired at ", acq.Acquired)
			}
		}
	}
}

func verifyTilesCountStatusByAcq(acq *models.Acquisition, session dao.DbSession) error {
	testCountsByAcqID := map[uint64]map[int64]map[string]int{
		acq.AcqUID: map[int64]map[string]int{
			100: map[string]int{
				"READY": 4,
			},
			101: map[string]int{
				"READY": 5,
			},
			102: map[string]int{
				"READY": 5,
			},
		},
	}
	var tileFilter models.TileFilter
	tileFilter.AcqUID = acq.AcqUID
	tileFilter.NominalSection = -1
	tileFilter.IncludeNotPersisted = true
	tileFilter.FrameType = models.Stable
	countsByStatus, err := dao.CountTilesByStatus(&tileFilter, session)
	if err != nil {
		return fmt.Errorf("Expected %v but instead it returned error %v", testCountsByAcqID, err)
	}
	if !reflect.DeepEqual(countsByStatus, testCountsByAcqID) {
		return fmt.Errorf("Expected to retrieve %v for %d but it got %v", testCountsByAcqID, acq.AcqUID, countsByStatus)
	}
	return nil
}

func verifyTilesCountStatusByAcqAndSection(acq *models.Acquisition, session dao.DbSession) error {
	testCountsBySection := map[int64]map[string]int{
		99: map[string]int{
			"nAcquisitions": 0,
			"nReadyTiles":   0,
		},
		100: map[string]int{
			"nAcquisitions": 1,
			"nSections":     1,
			"nReadyTiles":   4,
		},
		101: map[string]int{
			"nAcquisitions": 1,
			"nSections":     1,
			"nReadyTiles":   5,
		},
		102: map[string]int{
			"nAcquisitions": 1,
			"nSections":     1,
			"nReadyTiles":   5,
		},
		103: map[string]int{
			"nAcquisitions": 0,
			"nReadyTiles":   0,
		},
	}
	var tileFilter models.TileFilter
	tileFilter.AcqUID = acq.AcqUID
	tileFilter.NominalSection = -1
	var counts map[string]int
	for tileFilter.NominalSection, counts = range testCountsBySection {
		countsByStatus, err := dao.CountTilesByStatus(&tileFilter, session)
		if err != nil {
			return fmt.Errorf("Section %v expected %v but instead it returned error %v", tileFilter.NominalSection, counts, err)
		}
		acqIDs := make([]uint64, 0, len(countsByStatus))
		for k, _ := range countsByStatus {
			acqIDs = append(acqIDs, k)
		}

		if len(acqIDs) != counts["nAcquisitions"] {
			return fmt.Errorf("Expected to retrieve %d acquisitions, but instead it got %d", counts["nAcquisitions"], len(acqIDs))
		}
		if len(acqIDs) == 0 {
			continue
		}
		if !reflect.DeepEqual(acqIDs, []uint64{acq.AcqUID}) {
			return fmt.Errorf("Expected to retrieve data only for %d but instead it got %v", acq.AcqUID, acqIDs)
		}
		sections := countsByStatus[acq.AcqUID]
		if len(sections) != counts["nSections"] {
			return fmt.Errorf("Expected to retrieve %d sections, but instead it got %d", counts["nSections"], len(sections))
		}
		if sections[tileFilter.NominalSection]["READY"] != counts["nReadyTiles"] {
			return fmt.Errorf("Expected to retrieve %d ready tiles for %v but instead it got %d",
				counts["nReadyTiles"], tileFilter.NominalSection, sections[tileFilter.NominalSection]["READY"])
		}
	}
	return nil
}

func verifyTilesCountStatusBySample(sampleName string, acqs []*models.Acquisition, session dao.DbSession) error {
	testCountsByAcqID := map[uint64]map[int64]map[string]int{}
	for _, acq := range acqs {
		testCountsByAcqID[acq.AcqUID] = map[int64]map[string]int{
			100: map[string]int{
				"READY": 4,
			},
			101: map[string]int{
				"READY": 5,
			},
			102: map[string]int{
				"READY": 5,
			},
		}
	}
	var tileFilter models.TileFilter
	tileFilter.SampleName = sampleName
	tileFilter.NominalSection = -1
	countsByStatus, err := dao.CountTilesByStatus(&tileFilter, session)
	if err != nil {
		return fmt.Errorf("Expected %v but instead it returned error %v", testCountsByAcqID, err)
	}
	if !reflect.DeepEqual(countsByStatus, testCountsByAcqID) {
		return fmt.Errorf("Expected to retrieve %v for %v but it got %v", testCountsByAcqID, acqs, countsByStatus)
	}
	return nil
}

func TestRetrieveTileROI(t *testing.T) {
	var err error
	session, err := TestDBHandler.OpenSession(false)
	defer func() {
		session.Close(fmt.Errorf("Rollback everything"))
	}()
	if err != nil {
		log.Panic(err)
	}
	_, imageMosaic, err := createTestMosaic("Test sample", 100, session)
	if err != nil {
		log.Panic(err)
	}
	if _, _, err = createTestTiles(imageMosaic, "READY", session); err != nil {
		log.Panic(err)
	}
	var tileFilter models.TileFilter
	// retrieve stable tiles and check that all have a ROI
	tileFilter.ImageMosaicID = imageMosaic.ImageMosaicID
	tileFilter.FrameType = models.Stable
	tileFilter.Row = -1
	tileFilter.Col = -1
	tileFilter.CameraConfig.Camera = -1
	stableTiles, err := dao.RetrieveTemImages(&tileFilter, session)
	if err != nil {
		log.Panic(err)
	}
	if len(stableTiles) == 0 {
		t.Error("Expected non empty list of stable tiles")
	}
	for _, stableTile := range stableTiles {
		if stableTile.Roi.AcqRoiID == 0 {
			t.Error("Expected the Acq ROI ID to be set for stable tile", stableTile)
		}
		if stableTile.Roi.MosaicRoiID == 0 {
			t.Error("Expected the Mosaic ROI ID to be set for stable tile", stableTile)
		}
	}
	// retrieve drift tiles and check that none has a ROI
	tileFilter.FrameType = models.Drift
	tileFilter.IncludeNotPersisted = true
	driftTiles, err := dao.RetrieveTemImages(&tileFilter, session)
	if err != nil {
		log.Panic(err)
	}
	if len(driftTiles) == 0 {
		t.Error("Expected non empty list of drift tiles")
	}
	for _, driftTile := range driftTiles {
		if driftTile.Roi.AcqRoiID != 0 {
			t.Error("Did not expect the Acq ROI ID to be set for drift tile", driftTile)
		}
		if driftTile.Roi.MosaicRoiID != 0 {
			t.Error("Did not expect the Mosaic ROI ID to be set for drift tile", driftTile)
		}
		tileAcqROIs, err := dao.RetrieveTileROIsByMosaicColAndRow(driftTile.ImageMosaic.ImageMosaicID, driftTile.Col, driftTile.Row, session)
		if err != nil {
			t.Error("Unexpected error while retrieving the corresponding region of interest for", driftTile)
			continue
		}
		for _, tileAcqROI := range tileAcqROIs {
			if _, roiErr := dao.CreateTileROI(&driftTile.ImageMosaic, driftTile.ImageID, tileAcqROI, models.TileReadyState, session); roiErr != nil {
				t.Error("Unexpected error while associating region of interest", tileAcqROI, "with tile", driftTile)
			}
		}
	}
	// now retrieve them again and each drift tile should have a ROI
	updatedDriftTiles, err := dao.RetrieveTemImages(&tileFilter, session)
	if err != nil {
		log.Panic(err)
	}
	if len(updatedDriftTiles) < len(driftTiles) {
		t.Error("Expected the number of drift tiles to be the same as before or greater considering that a tile can be associated with more than one ROI",
			len(driftTiles), "instead", len(updatedDriftTiles))
	}
	for _, driftTile := range updatedDriftTiles {
		if driftTile.Roi.AcqRoiID == 0 {
			t.Error("Expected the Acq ROI ID to be set for drift tile", driftTile)
		}
		if driftTile.Roi.MosaicRoiID == 0 {
			t.Error("Expected the Mosaic ROI ID to be set for drift tile", driftTile)
		}
	}

}
