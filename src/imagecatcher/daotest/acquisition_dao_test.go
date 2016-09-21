package daotest

import (
	"fmt"
	"log"
	"testing"
	"time"

	"imagecatcher/dao"
	"imagecatcher/models"
)

func init() {
	InitTestDB()
}

func TestAcqFilteringByTileInStateParams(t *testing.T) {
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
	var nTiles int
	var nUpdates int64
	testStartState := "TEST READY"
	testEndState := "TEST DONE"
	for i := 0; i < nacqs; i++ {
		acq, imageMosaic, err := createTestMosaic(sampleName, 100+uint64(i), session)
		if err != nil {
			log.Panic(err)
		}
		if nTiles, _, err = createTestTiles(imageMosaic, testStartState, session); err != nil {
			log.Panic(err)
		}
		if nTiles != 14 { // testTileROIs has 14 valid ROIs
			t.Error("Expected to create", 14, "tiles but instead it created", nTiles)
		}
		acqs[i] = acq
		if i == 0 {
			continue
		}
		tileFilter := &models.TileFilter{
			TileSpec: models.TileSpec{
				Acquisition:    *acq,
				NominalSection: -1,
				Col:            -1,
				Row:            -1,
				State:          testStartState,
				CameraConfig: models.TemCameraConfiguration{
					Camera: -1,
				},
			},
			FrameType:           models.Stable,
			IncludeNotPersisted: true,
			Pagination:          models.Page{0, int32(i)},
		}
		retrievedTiles, err := dao.RetrieveTemImages(tileFilter, session)
		if err != nil {
			t.Error("Unexpected error retrieving tile images for acquisition", acq.AcqUID, "using filter", tileFilter, err)
		}
		tilesInStartState, err := dao.UpdateTilesStatus(tileFilter, testStartState, session)
		if err != nil {
			t.Error("Unexpected error retrieving tiles in ", testStartState, "for acquisition ", acq.AcqUID, err)
		}
		if len(tilesInStartState) != len(retrievedTiles) {
			t.Error("Expected the number of updated tiles to be equal to the retrieved tiles using the same filter: ", len(retrievedTiles),
				"but it got", len(tilesInStartState))
		}
		if nUpdates, err = dao.UpdateTemImageROIState(tilesInStartState, testEndState, session); err != nil {
			t.Error("Unexpected error while updating the state to ", testEndState, "for", tileFilter, err)
		}
		if int(nUpdates) != i {
			t.Error("Expected to update", i, "tiles but instead it updated", nUpdates)
		}
	}
	acqParams := models.Acquisition{
		SampleName: sampleName,
	}
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(24 * time.Hour)
	type testDataType struct {
		acqFilter      models.AcquisitionFilter
		expectedCounts int
	}
	tests := []testDataType{
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 1},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, nacqs},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, &to},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 1},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, &to},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, nacqs},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&from, nil},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 1},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&from, nil},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, nacqs},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&from, &to},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 1},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&from, &to},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, nacqs},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&to, nil},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&to, nil},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, &from},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: "",
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, &from},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testStartState,
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       testEndState,
			RequiredStateForAtLeastOneTile: "",
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: testEndState,
		}, nacqs - 1},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       "invalid",
			RequiredStateForAtLeastOneTile: "",
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{nil, nil},
			RequiredStateForAllTiles:       "",
			RequiredStateForAtLeastOneTile: "invalid",
		}, 0},
		{models.AcquisitionFilter{
			Acquisition:                    acqParams,
			AcquiredInterval:               models.TimeInterval{&from, &to},
			RequiredStateForAllTiles:       testStartState,
			RequiredStateForAtLeastOneTile: testEndState,
		}, 0},
	}
	for ti, testData := range tests {
		retrievedAcqs, err := dao.RetrieveAcquisitions(&testData.acqFilter, session)
		if err != nil {
			t.Error(ti, "Unexpected error while retrieving acquisitions with filter", testData.acqFilter, err)
		}
		if len(retrievedAcqs) != testData.expectedCounts {
			t.Error(ti, "Expected", testData.expectedCounts, "for filter", testData.acqFilter, "but it got", len(retrievedAcqs), retrievedAcqs)
		}
	}
}
