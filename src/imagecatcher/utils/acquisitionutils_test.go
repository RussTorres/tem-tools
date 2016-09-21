package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestGetAcqUidFromUrl(t *testing.T) {
	type testdata struct {
		input          string
		expectedResult uint64
		expectedError  error
	}
	var tests = []testdata{
		{"123_foo", 123, nil},
		{"/123_foo", 123, nil},
		{"/foo/123_bar", 123, nil},
		{"123", 123, nil},
		{"_123", 0, fmt.Errorf("Invalid name component - missing ID before '_'")},
		{"/foo_bar", 0, fmt.Errorf(`Error parsing acquisition UID: strconv.ParseUint: parsing "foo": invalid syntax`)},
		{"/123foo_bar", 0, fmt.Errorf(`Error parsing acquisition UID: strconv.ParseUint: parsing "123foo": invalid syntax`)},
	}
	for _, data := range tests {
		acqID, err := GetAcqUIDFromURL(data.input)
		if acqID != data.expectedResult ||
			data.expectedError == nil && err != nil ||
			data.expectedError != nil && err.Error() != data.expectedError.Error() {
			t.Error(
				"For", data.input,
				"expected", data.expectedResult, data.expectedError,
				"got", acqID, err,
			)
		}
	}
}

func TestParseROITiles(t *testing.T) {
	roiTilesFilename := "testdata/160517112929_30x60_Sect_001_ROI_tiles.csv"
	f, err := os.Open(roiTilesFilename)
	if err != nil {
		t.Error("Error opening", roiTilesFilename, " ", err)
	}
	defer f.Close()
	tiles, err := parseROITiles(f)
	if err != nil {
		t.Error("Error parsing ", roiTilesFilename, " ", err)
	}
	if len(tiles) != 7200 {
		t.Error("Expected 7200 tiles but found only ", len(tiles))
	}
	var expectedSection int64
	for i, tile := range tiles {
		expectedCol := (i / 4) / 60
		expectedRow := (i / 4) % 60
		expectedSection = 1
		if tile.Row != expectedRow && tile.Col != expectedCol && tile.Roi.NominalSection != expectedSection {
			t.Error("Expected col,row,section ",
				expectedCol, expectedRow, expectedSection,
				" but got ",
				tile.Col, tile.Row, tile.Roi.NominalSection)
		}
	}
}

func TestParseROISpec(t *testing.T) {
	roiSpecFilename := "testdata/160517112929_30x60_Sect_001_ROI_spec.ini"
	f, err := os.Open(roiSpecFilename)
	if err != nil {
		t.Error("Error opening", roiSpecFilename, " ", err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		t.Error("Error opening", roiSpecFilename, " ", err)
	}
	zSectionByRegion, err := ParseTileSpec(content)
	if err != nil {
		t.Error("Error parsing", roiSpecFilename, " ", err)
	}
	if len(zSectionByRegion) != 1 {
		t.Error("Expected only 1 region in ", roiSpecFilename)
	}
}

func TestExtractNominalSection(t *testing.T) {
	type testdata struct {
		input          string
		expectedResult int64
		expectedError  error
	}
	var tests = []testdata{
		{".A.001", 1, nil},
		{"1", 1, nil},
		{".123", 123, nil},
		{"456.B.123", 123, nil},
		{".A", 0, fmt.Errorf(`strconv.ParseInt: parsing "A": invalid syntax`)},
	}
	for _, data := range tests {
		section, err := extractNominalSectionFromSectionID(data.input)
		if section != data.expectedResult ||
			data.expectedError == nil && err != nil ||
			data.expectedError != nil && err.Error() != data.expectedError.Error() {
			t.Error(
				"For", data.input,
				"expected", data.expectedResult, data.expectedError,
				"got", section, err,
			)
		}
	}
}
