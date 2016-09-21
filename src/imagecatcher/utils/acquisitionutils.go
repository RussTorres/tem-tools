package utils

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"imagecatcher/logger"
	"imagecatcher/models"
)

// GetAcqUIDFromURL extract acquisition id from a url
func GetAcqUIDFromURL(url string) (uint64, error) {
	urlcomponents := strings.Split(url, "/")
	name := urlcomponents[len(urlcomponents)-1]
	sepIndex := strings.Index(name, "_")
	if sepIndex == 0 {
		return 0, errors.New("Invalid name component - missing ID before '_'")
	}
	if sepIndex > 0 {
		name = name[:sepIndex]
	}
	if name == "" {
		return 0, fmt.Errorf("Cannot extract acquisition UID from an empty URL")
	}
	acqUID, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return acqUID, fmt.Errorf("Error parsing acquisition UID: %v", err)
	}
	return acqUID, err
}

// ParseAcqIniLog - parses an INI file
func ParseAcqIniLog(acqIniLogContent []byte) (*models.Acquisition, error) {
	acq := &models.Acquisition{}

	acqIniLog, err := LoadIniBuffer(acqIniLogContent)
	if err != nil {
		return acq, err
	}
	acq.IniContent = string(acqIniLogContent)
	generalSection := acqIniLog.Section("General")
	acq.SampleName = strings.Trim(generalSection.Get("Sample name"), `"`)
	acq.MicroscopistName = strings.Trim(generalSection.Get("Microscopist name"), `"`)
	acq.Notes = strings.Trim(generalSection.Get("Notes"), `"`)

	date := strings.Trim(generalSection.Get("Date"), `"`)
	acq.Acquired, err = time.Parse("01/02/2006 03:04:05 PM", date)
	acq.Inserted = time.Now()
	acq.Completed = nil

	cameraConnectionsSection := acqIniLog.Section("Camera Connections")
	if cameraConnectionsSection.Get("Acq1") == "TRUE" &&
		cameraConnectionsSection.Get("Acq2") == "TRUE" &&
		cameraConnectionsSection.Get("Acq3") == "TRUE" &&
		cameraConnectionsSection.Get("Acq4") == "TRUE" {
		acq.NumberOfCameras = 4
	} else if cameraConnectionsSection.Get("Acq1") == "TRUE" &&
		cameraConnectionsSection.Get("Acq2") == "FALSE" &&
		cameraConnectionsSection.Get("Acq3") == "FALSE" &&
		cameraConnectionsSection.Get("Acq4") == "FALSE" {
		acq.NumberOfCameras = 1
	} else {
		err = fmt.Errorf("Invalid camera connections (%s, %s, %s, %s) ", cameraConnectionsSection.Get("Acq1"),
			cameraConnectionsSection.Get("Acq2"), cameraConnectionsSection.Get("Acq3"), cameraConnectionsSection.Get("Acq4"))
		return acq, err
	}

	mosaicSettings := acqIniLog.Section("Mosaic Settings")
	if acq.XSmallStepPix, err = strconv.Atoi(mosaicSettings.Get("X sm step (pix)")); err != nil {
		logger.Errorf("Error parsing X small step %s: %v", mosaicSettings.Get("X sm step (pix)"), err)
	}
	if acq.YSmallStepPix, err = strconv.Atoi(mosaicSettings.Get("Y sm step (pix)")); err != nil {
		logger.Errorf("Error parsing Y small step %s: %v", mosaicSettings.Get("Y sm step (pix)"), err)
	}
	if acq.XBigStepPix, err = strconv.Atoi(mosaicSettings.Get("X big step (pix)")); err != nil {
		logger.Errorf("Error parsing X big step %s: %v", mosaicSettings.Get("X big step (pix)"), err)
	}
	if acq.YBigStepPix, err = strconv.Atoi(mosaicSettings.Get("Y big step (pix)")); err != nil {
		logger.Errorf("Error parsing Y big step %s: %v", mosaicSettings.Get("Y big step (pix)"), err)
	}
	if acq.XCenter, err = strconv.ParseFloat(mosaicSettings.Get("Center X (m)"), 64); err != nil {
		logger.Errorf("Error parsing X center %s: %v", mosaicSettings.Get("Center X (m)"), err)
	}
	if acq.YCenter, err = strconv.ParseFloat(mosaicSettings.Get("Center Y (m)"), 64); err != nil {
		logger.Errorf("Error parsing Y center %s: %v", mosaicSettings.Get("Center Y (m)"), err)
	}
	if acq.NXSteps, err = strconv.Atoi(mosaicSettings.Get("Number of X steps")); err != nil {
		logger.Errorf("Error parsing number of X steps %s: %v", mosaicSettings.Get("Number of X steps"), err)
	}
	if acq.NYSteps, err = strconv.Atoi(mosaicSettings.Get("Number of Y steps")); err != nil {
		logger.Errorf("Error parsing number of Y steps %s: %v", mosaicSettings.Get("Number of Y steps"), err)
	}

	if acq.NumberOfCameras == 1 {
		acq.NTargetCols = acq.NXSteps
		acq.NTargetRows = acq.NYSteps
	} else if acq.NumberOfCameras == 4 {
		acq.NTargetCols = 2 * acq.NXSteps
		acq.NTargetRows = 2 * acq.NYSteps
	} else {
		err = fmt.Errorf("Number of cameras must be 1 or 4 - got %d", acq.NumberOfCameras)
	}

	temSettings := acqIniLog.Section("TEM Settings")
	magnification := strings.Trim(temSettings.Get("magnification"), `"`)
	if acq.Magnification, err = strconv.Atoi(magnification); err != nil {
		logger.Errorf("Error parsing magnification %s: %v", temSettings.Get("magnification"), err)
	}
	pixelsPerUm := strings.Trim(temSettings.Get("pixels per um"), `"`)
	if acq.PixPerUm, err = strconv.ParseFloat(pixelsPerUm, 64); err != nil {
		logger.Errorf("Error parsing pixels per um %s: %v", temSettings.Get("pixels per um"), err)
	}

	machineIdsSection := acqIniLog.Section("Machine IDs")
	acq.TemIPAddress = strings.Trim(machineIdsSection.Get("TEM IP address"), `"`)

	acqSettings := acqIniLog.Section("ImageCatcher")
	acq.ProjectName = strings.Trim(acqSettings.Get("project"), `"`)
	acq.ProjectOwner = strings.Trim(acqSettings.Get("owner"), `"`)
	acq.StackName = strings.Trim(acqSettings.Get("stack"), `"`)
	acq.MosaicType = strings.Trim(acqSettings.Get("mosaicType"), `"`)

	return acq, err
}

// ParseTileSpec - parses a tile spec INI file and returns a mapping of region numbers to nominal sections
func ParseTileSpec(tileSpecContent []byte) (map[string]*models.AcqROI, error) {
	regionToAcqROI := make(map[string]*models.AcqROI)

	tileSpec, err := LoadIniBuffer(tileSpecContent)
	if err != nil {
		return regionToAcqROI, err
	}
	inclusionSectionNumbers := tileSpec.Section("Inclusion Section Numbers")

	var nominalSection int64
	var parseSectionErr error
	for region, sectionName := range inclusionSectionNumbers.GetAll() {
		region := strings.TrimPrefix(strings.ToLower(region), "region ")
		section := strings.Trim(sectionName, `"`)
		if nominalSection, err = extractNominalSectionFromSectionID(section); err != nil {
			parseSectionErr = fmt.Errorf("Error extracting the nominal section value from region: %s (%s)", region, sectionName)
			logger.Error(parseSectionErr)
			nominalSection = 0
		}
		regionToAcqROI[region] = &models.AcqROI{
			RegionName:     region,
			NominalSection: nominalSection,
			SectionName:    section,
		}
	}
	return regionToAcqROI, parseSectionErr
}

// ParseROITiles - parses a CSV ROI tiles file
func ParseROITiles(roiTilesContent []byte) ([]models.TemImageROI, error) {
	return parseROITiles(bytes.NewReader(roiTilesContent))
}

func parseROITiles(roiTilesReader io.Reader) ([]models.TemImageROI, error) {
	var roiTiles []models.TemImageROI
	csvReader := csv.NewReader(roiTilesReader)
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1

	csvRecords, err := csvReader.ReadAll()
	if err != nil {
		logger.Errorf("Error while parsing the ROI Tiles: %s", err)
		return roiTiles, err
	}
	headers := make(map[string]int)
	for i, h := range csvRecords[0] {
		headers[h] = i
	}
	for _, r := range csvRecords[1:] {
		var ti models.TemImageROI
		if len(r) == 1 {
			continue
		}
		ti.Roi.RegionName = r[headers["Inclusion"]]

		regionNo, err := strconv.ParseInt(ti.Roi.RegionName, 10, 64)
		if err != nil {
			logger.Errorf("WARNING: Invalid inclusion region: %s in %v", ti.Roi.RegionName, r)
			continue
		} else if regionNo < 0 {
			continue // skip region -1
		}
		ti.Roi.SectionName = r[headers["Section number"]]
		ti.Roi.NominalSection, err = extractNominalSectionFromSectionID(ti.Roi.SectionName)
		if err != nil {
			logger.Errorf("WARNING: Invalid section number: %s in %v", r[headers["Section number"]], r)
			continue
		}
		ti.Col, err = strconv.Atoi(r[headers["Col"]])
		if err != nil {
			logger.Errorf("WARNING: Invalid column number: %s in %v", r[headers["Col"]], r)
			continue
		}
		ti.Row, err = strconv.Atoi(r[headers["Row"]])
		if err != nil {
			logger.Errorf("WARNING: Invalid row number: %s in %v", r[headers["Row"]], r)
			continue
		}
		ti.Frame = -1
		roiTiles = append(roiTiles, ti)
	}
	return roiTiles, err
}

func extractNominalSectionFromSectionID(sectionIDVal string) (int64, error) {
	sectionSubFields := strings.Split(sectionIDVal, ".")
	sectionNum := sectionSubFields[len(sectionSubFields)-1]
	return strconv.ParseInt(sectionNum, 10, 64)
}
