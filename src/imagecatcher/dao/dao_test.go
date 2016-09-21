package dao

import (
	"testing"
)

func TestScanningNullAndValidTimeValues(t *testing.T) {
	var tests []interface{}
	tests = []interface{}{
		nil,
		[]byte("2013-06-12 03:04:04"),
		nil,
		[]byte("2011-07-11 15:04:04"),
		nil,
	}
	var nt NullableTime

	for i, test := range tests {
		if err := nt.Scan(test); err != nil {
			t.Error("Test", i, "unexpected error", err)
		}
		if test == nil && (nt.Valid || !nt.Time.IsZero()) {
			t.Error("Test", i, "nil value should not generate a valid scan", nt.Time, nt.Time.IsZero())
		}
		if test != nil && !nt.Valid && nt.Time.Format("2006-01-02 15:04:05") != string(test.([]byte)) {
			t.Error("Test", i, "Expected a valid scan for", test, "but it got", nt.Time)
		}
	}
}
