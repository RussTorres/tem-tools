package models

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateJFSAndPathParams(t *testing.T) {
	type testdata struct {
		jfsParams       map[string]interface{}
		expectedError   error
		expectedJfsPath string
		expectedJfsKey  string
	}
	var tests = []testdata{
		{
			map[string]interface{}{
				"k1": "v1",
			},
			errors.New("Error encountered while updating JFS parameters"),
			"",
			"",
		},
		{
			map[string]interface{}{
				"jfsPath": "p1",
			},
			errors.New("Error encountered while updating JFS parameters"),
			"p1",
			"",
		},
		{
			map[string]interface{}{
				"jfsPath":    "p1",
				"scalityKey": "s1",
			},
			nil,
			"p1",
			"s1",
		},
		{
			map[string]interface{}{
				"jfsPath":    "p1",
				"scalityKey": "s1",
				"checksum":   "foo",
			},
			errors.New("Error decoding the checksum foo: encoding/hex: odd length hex string"),
			"p1",
			"s1",
		},
		{
			map[string]interface{}{
				"jfsPath":    "p1",
				"scalityKey": "s1",
				"checksum":   "00112233445566778899AABBCCDDEEFF",
			},
			nil,
			"p1",
			"s1",
		},
	}
	for ti, testparams := range tests {
		f := FileObject{}
		err := f.UpdateJFSAndPathParams("test", testparams.jfsParams)
		if testparams.expectedError == nil && err != nil ||
			testparams.expectedError != nil && !strings.HasPrefix(err.Error(), testparams.expectedError.Error()) {
			t.Error("Test", ti, "For", testparams.jfsParams,
				"Expected error", testparams.expectedError,
				"got", err,
			)
		}
		if f.JfsKey != testparams.expectedJfsKey {
			t.Error("Test", ti, "Expected JfsKey", testparams.expectedJfsKey,
				"for", testparams.jfsParams,
			)
		}
		if f.JfsPath != testparams.expectedJfsPath {
			t.Error("Test", ti, "Expected JfsKey", testparams.expectedJfsPath,
				"for", testparams.jfsParams,
			)
		}
	}
}
