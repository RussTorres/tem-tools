package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"imagecatcher/logger"
)

// Config holds the configuration
type Config map[string]interface{}

// GetConfig initialize a config from the given filenames.
func GetConfig(cfgFileNames ...string) (*Config, error) {
	cfg := &Config{}
	for _, cfgFileName := range cfgFileNames {
		if err := cfg.readConfig(cfgFileName); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func (cfg *Config) readConfig(cfgFileName string) error {
	cfgContent, err := ioutil.ReadFile(cfgFileName)
	if err != nil {
		logger.Infof("Error reading config file %s: %v", cfgFileName, err)
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(cfgContent))
	var config map[string]interface{}
	err = decoder.Decode(&config)
	if err != nil {
		logger.Infof("Error reading JSON from config file %s: %v", cfgFileName, err)
		return err
	}
	for k, v := range config {
		(*cfg)[k] = v
	}
	return nil
}

// GetBoolProperty read property as a boolean
func (cfg Config) GetBoolProperty(name string, defaultValue bool) bool {
	if cfg[name] != nil {
		return cfg[name].(bool)
	}
	return defaultValue
}

// GetIntProperty read property as an integer
func (cfg Config) GetIntProperty(name string, defaultValue int) int {
	if cfg[name] != nil {
		switch v := cfg[name].(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		default:
			logger.Info("Expected int value for %s: %v - return default value %d", name, v, defaultValue)
		}
	}
	return defaultValue
}

// GetInt64Property read the property as a 64bit integer
func (cfg Config) GetInt64Property(name string, defaultValue int64) int64 {
	if cfg[name] != nil {
		switch v := cfg[name].(type) {
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		default:
			logger.Info("Expected int64 value for %s: %v - return default value %d", name, v, defaultValue)
		}
	}
	return defaultValue
}

// GetFloat64Property read the property as a 64bit float
func (cfg Config) GetFloat64Property(name string, defaultValue float64) float64 {
	if cfg[name] != nil {
		switch v := cfg[name].(type) {
		case int:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case float64:
			return v
		default:
			logger.Info("Expected float64 value for %s: %v - return default value %f", name, v, defaultValue)
		}
	}
	return defaultValue
}

// GetStringProperty - read property as a string
func (cfg Config) GetStringProperty(name string, defaultValue string) string {
	if cfg[name] != nil {
		return cfg[name].(string)
	}
	return defaultValue
}

// GetStringArrayProperty - read a string array property
func (cfg Config) GetStringArrayProperty(name string) (res []string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Error encountered while reading string array property: %s - %v", name, r)
		}
	}()
	switch v := cfg[name].(type) {
	case []string:
		res = v
	case string:
		res = []string{v}
	case []interface{}:
		res = make([]string, len(v))
		for i, vi := range v {
			res[i] = vi.(string)
		}
	case interface{}:
		res = []string{v.(string)}
	default:
		res = []string{}
	}
	return res
}

// GetStringMapProperty - read a map from string to string property
func (cfg Config) GetStringMapProperty(name string) (res map[string]string) {
	res = map[string]string{}
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Error encountered while reading string map property: %s - %v", name, r)
		}
	}()
	if cfg[name] != nil {
		for k, v := range cfg[name].(map[string]interface{}) {
			res[k] = v.(string)
		}
	}
	return res
}
