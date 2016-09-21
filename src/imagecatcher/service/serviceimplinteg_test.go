package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"imagecatcher/config"
	"imagecatcher/daotest"
)

func init() {
	daotest.InitTestDB()
}

func readFileParams(fileName string) (fp *FileParams, err error) {
	var f *os.File
	fp = &FileParams{Name: fileName}
	if f, err = os.Open(fileName); err != nil {
		return fp, err
	}
	if fp.Content, err = ioutil.ReadAll(f); err != nil {
		return fp, err
	}
	return fp, err
}

func importAcquisition(s ImageCatcherService) (uint64, error) {
	acqLogParams, err := readFileParams(testAcqLog)
	if err != nil {
		return 0, err
	}
	acq, err := s.CreateAcquisition(acqLogParams)
	return acq.AcqUID, err
}

func TestCreateROIs(t *testing.T) {
	testConfig := config.Config{}
	testJfsService := createJfsService()
	s := &imageCatcherServiceImpl{
		config:    testConfig,
		dbHandler: daotest.TestDBHandler,
	}
	s.initializeJFS(fakeJfsFactory{testJfsService})

	acqID, err := importAcquisition(s)
	if err != nil {
		log.Panic(err)
	}
	roiSpec, err := readFileParams(testRoiSpec)
	if err != nil {
		log.Panic(err)
	}
	roiTiles, err := readFileParams(testRoiTiles)
	if err != nil {
		log.Panic(err)
	}
	roiFiles := &ROIFiles{
		RoiTiles: *roiTiles,
		RoiSpec:  *roiSpec,
	}
	acqMosaic, err := s.GetMosaic(acqID)
	if err != nil {
		log.Panic(err)
	}
	err = s.CreateROIs(acqMosaic, roiFiles)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if testJfsService.storage.content.Len() != 2 {
		t.Error("Expected 2 items to have been sent to JFS - got only ", testJfsService.storage.content.Len())
	}
	storedRoiSpec := testJfsService.storage.content.Front().Value.(FileParams)
	storedRoiTiles := testJfsService.storage.content.Front().Next().Value.(FileParams)
	expectedRoiSpecName := fmt.Sprintf("/acquisitions/%d/%s", acqID, testRoiSpec)
	expectedRoiTilesName := fmt.Sprintf("/acquisitions/%d/%s", acqID, testRoiTiles)
	if storedRoiSpec.Name != expectedRoiSpecName {
		t.Error("For ROI spec expected", expectedRoiSpecName, "got", storedRoiSpec.Name)
	}
	if storedRoiTiles.Name != expectedRoiTilesName {
		t.Error("For ROI tiles expected", expectedRoiTilesName, "got", storedRoiTiles.Name)
	}
	if !bytes.Equal(storedRoiSpec.Content, roiSpec.Content) {
		t.Error("For ROI spec the content to be equal")
	}
	if !bytes.Equal(storedRoiTiles.Content, roiTiles.Content) {
		t.Error("For ROI tiles the content to be equal")
	}
}

func TestStoreTileInfo(t *testing.T) {
	testConfig := config.Config{}
	testJfsService := createJfsService()
	s := &imageCatcherServiceImpl{
		config:    testConfig,
		dbHandler: daotest.TestDBHandler,
	}
	s.initializeJFS(fakeJfsFactory{testJfsService})

	acqID, err := importAcquisition(s)
	if err != nil {
		t.Error("Unexpected error while importing acquisition", err)
	}
	tileParams := &TileParams{
		Col: 121212, Row: 212121,
	}
	acqMosaic, err := s.GetMosaic(acqID)
	if err != nil {
		t.Error("Failed to retrieve acquisition mosaic for", acqID, err)
	}
	tile, err := s.StoreTile(acqMosaic, tileParams)
	if err != nil {
		t.Error("Unexpected error for", tileParams, err)
	}
	if tile.Col != tileParams.Col || tile.Row != tileParams.Row {
		t.Error(
			"For", tileParams,
			"expected",
			"col", tileParams.Col, "row", tileParams.Row,
			"got",
			"col", tile.Col, "row", tile.Row,
		)
	}
	if tile.Configuration == nil {
		t.Error("Expected tile configuration to be set ")
	} else if tile.Configuration.Camera != tileParams.Camera {
		t.Error("Expected camera", tileParams.Camera, "got", tile.Configuration.Camera)
	}
}

func TestExtractAncillaryFileType(t *testing.T) {
	testData := map[string]string{
		"50407200041_40x168_Add pulse.csv":               pulseFileType,
		"150407200041_40x168_Controller elapsed.csv":     controllerElapsedFileType,
		"150407200041_40x168_Controller timing.csv":      controllerTimingFileType,
		"150407200041_40x168_DAQ timing.csv":             daqTimingFileType,
		"150407200041_40x168_ROI_spec.ini":               roiSpecFileType,
		"150407200041_40x168_ROI_tiles.csv":              roiTilesFileType,
		"150407200041_40x168_logfile.ini":                logfileFileType,
		"150407200041_40x168_mosaic.png":                 mosaicFileType,
		"150407200041_40x168_statistics_cam0.csv":        statisticsFileType,
		"150407200041_40x168_statistics_cam1.csv":        statisticsFileType,
		"150407200041_40x168_statistics_cam2.csv":        statisticsFileType,
		"150407200041_40x168_statistics_cam3.csv":        statisticsFileType,
		"150407200041_40x168_stored_image_list_cam0.csv": storedImageListFileType,
		"150407200041_40x168_stored_image_list_cam1.csv": storedImageListFileType,
		"150407200041_40x168_stored_image_list_cam2.csv": storedImageListFileType,
		"150407200041_40x168_stored_image_list_cam3.csv": storedImageListFileType,
		"150407200041_40x168_stored_image_list_cam4.csv": unknownFileType,
	}
	for input, expected := range testData {
		result := extractAncillaryFileType(input)
		if result != expected {
			t.Error("For", input, "expected", expected, "but got", result)
		}
	}
}
