package utils

import (
	"bytes"
	"github.com/vaughan0/go-ini"
	"strings"

	"imagecatcher/logger"
)

// IniFile internal representation of an INI file
type IniFile struct {
	IniSections ini.File
}

// Section it's basically an ini.Section but the keys are all lower case
type Section struct {
	m map[string]string
}

// LoadIniBuffer - create the internal representation of an INI file from a blob
func LoadIniBuffer(content []byte) (*IniFile, error) {
	ini, err := ini.Load(bytes.NewReader(content))
	if err != nil {
		logger.Infof("Error loading ini content %s", err)
		return nil, err
	}
	iniFile := new(IniFile)
	iniFile.IniSections = ini
	return iniFile, nil
}

// Section extracts the corresponding section from an INI file
func (f IniFile) Section(name string) Section {
	s := Section{m: make(map[string]string)}
	for k, v := range f.IniSections.Section(name) {
		s.m[strings.ToLower(k)] = v
	}
	return s
}

// Get get the value for the corresponding key where the key is case insensitive
func (s Section) Get(k string) string {
	return s.m[strings.ToLower(k)]
}

// GetAll returns all key values from the section
func (s Section) GetAll() map[string]string {
	return s.m
}
