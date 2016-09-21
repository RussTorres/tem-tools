package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"imagecatcher/dao"
	"imagecatcher/models"
)

// TransformationSpec - JSON transformation representation
type TransformationSpec struct {
	ClassName  string `json:"className"`
	DataString string `json:"dataString"`
}

// CalibrationSpec - JSON calibration representation
type CalibrationSpec struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Metadata struct {
		CalibrationDate string `json:"calibration_data"`
		Group           string `json:"group"`
		Temca           string `json:"temca"`
		Camera          string `json:"camera"`
	}
	SpecList []TransformationSpec `json:"specList"`
}

// CalibrationJSONReader reads calibrations from a JSON stream
type CalibrationJSONReader struct {
}

// Read the calibration data from the given stream.
func (cr CalibrationJSONReader) Read(r io.Reader) ([]*models.Calibration, error) {
	decoder := json.NewDecoder(r)

	var cms []*models.Calibration
	var calibrations []CalibrationSpec
	var err error
	if err = decoder.Decode(&calibrations); err != nil {
		return cms, fmt.Errorf("Error reading calibrations as JSON: %v", err)
	}

	var camera int
	var temcaID int64
	var calibrationBuff bytes.Buffer
	for ci, c := range calibrations {
		var calibrationDate time.Time
		if calibrationDate, err = time.Parse("060102", c.Metadata.CalibrationDate); err != nil {
			return cms, fmt.Errorf("Record %d: Wrong calibration date %v: %v", ci, c.Metadata.CalibrationDate, err)
		}
		if err = json.NewEncoder(&calibrationBuff).Encode(c); err != nil {
			return cms, fmt.Errorf("Record %d: Error encoding calibration '%v' as JSON for storage: %v", ci, c, err)
		}
		if camera, err = strconv.Atoi(c.Metadata.Camera); err != nil {
			return cms, fmt.Errorf("Record %d: Expected a number for camera number but it got %v", ci, c.Metadata.Camera)
		}
		if temcaID, err = strconv.ParseInt(c.Metadata.Temca, 10, 64); err != nil {
			return cms, fmt.Errorf("Record %d: Expected a number for temca ID but it got %v", ci, c.Metadata.Temca)
		}

		cm := &models.Calibration{
			Name:        c.ID,
			Generated:   calibrationDate,
			JSONContent: calibrationBuff.String(),
			TemcaID:     temcaID,
			Camera:      camera,
		}
		cms = append(cms, cm)
		calibrationBuff.Reset()
	}

	return cms, nil
}

// Configurator is in charge with importing/updating various system settings.
type Configurator interface {
	GetCalibrations(pagination models.Page) ([]*models.Calibration, error)
	GetCalibrationsByName(name string) ([]*models.Calibration, error)
	ImportCalibrations(cs []*models.Calibration) error
	UpdateMaskURL(temcaID int64, camera int, rootURL string) error
}

type configuratorImpl struct {
	dbh dao.DbHandler
}

// NewConfigurator - instantiante a system configurator
func NewConfigurator(dbHandler dao.DbHandler) Configurator {
	return &configuratorImpl{dbh: dbHandler}
}

// GetCalibrations implements the corresponding Configurator method.
func (cfgi *configuratorImpl) GetCalibrations(pagination models.Page) (calibrations []*models.Calibration, err error) {
	var session dao.DbSession
	session, _ = cfgi.dbh.OpenSession(true)
	calibrations, err = dao.RetrieveAllCalibrations(pagination, session)
	session.Close(err)
	return calibrations, err
}

// GetCalibrationsByName implements the corresponding Configurator method.
func (cfgi *configuratorImpl) GetCalibrationsByName(name string) (calibrations []*models.Calibration, err error) {
	var session dao.DbSession
	session, _ = cfgi.dbh.OpenSession(true)
	calibrations, err = dao.RetrieveCalibrationByName(name, session)
	session.Close(err)
	return calibrations, err
}

// ImportCalibrations implements the corresponding Configurator method.
func (cfgi *configuratorImpl) ImportCalibrations(calibrations []*models.Calibration) (err error) {
	var session dao.DbSession
	if session, err = cfgi.dbh.OpenSession(false); err != nil {
		return fmt.Errorf("Error creating a database session: %v", err)
	}
	for _, c := range calibrations {
		if dbErr := dao.CreateOrUpdateCalibration(c, session); dbErr != nil {
			err = fmt.Errorf("Error creating or updating the calibration entry for %v: %v", c, dbErr)
			break
		}
		if c.TemcaID < 0 || c.Camera < 0 {
			err = fmt.Errorf("Invalid temca and/or camera for %v", c)
			break
		}
		cameraConfigs, dbErr := dao.RetrieveCameraConfigurationsByTemcaIDAndCameraNo(c.TemcaID, c.Camera, session)
		if dbErr != nil {
			err = fmt.Errorf("Error retrieving camera configuration for %d - %d: %v", c.TemcaID, c.Camera, dbErr)
			break
		}
		if len(cameraConfigs) != 1 {
			err = fmt.Errorf("Expected exactly one camera configuration for %d - %d but it got %d", c.TemcaID, c.Camera, len(cameraConfigs))
			break
		}
		camConfig := cameraConfigs[0]
		camConfig.TransformationRef = c.Name
		if dbErr := dao.UpdateCameraConfiguration(camConfig, session); dbErr != nil {
			err = fmt.Errorf("Error updating the camera configuration for %d - %d : %v", c.TemcaID, c.Camera, dbErr)
			break
		}
	}
	session.Close(err)
	return nil
}

// UpdateMaskURL implements the corresponding Configurator method.
func (cfgi *configuratorImpl) UpdateMaskURL(temcaID int64, camera int, rootURL string) (err error) {
	var session dao.DbSession
	if session, err = cfgi.dbh.OpenSession(false); err != nil {
		return fmt.Errorf("Error creating a database session: %v", err)
	}
	defer func() {
		session.Close(err)
	}()
	cameraConfigs, dbErr := dao.RetrieveCameraConfigurationsByTemcaIDAndCameraNo(temcaID, camera, session)
	if dbErr != nil {
		err = fmt.Errorf("Error retrieving camera configuration for %d - %d: %v", temcaID, camera, dbErr)
		return
	}
	if len(cameraConfigs) != 1 {
		err = fmt.Errorf("Expected exactly one camera configuration for %d - %d but it got %d", temcaID, camera, len(cameraConfigs))
		return
	}
	camConfig := cameraConfigs[0]
	if rootURL == "" {
		return
	}
	camConfig.MaskURL = fmt.Sprintf("%s/temca%d_cam%d.png", rootURL, temcaID, camera)
	if dbErr := dao.UpdateCameraConfiguration(camConfig, session); dbErr != nil {
		err = fmt.Errorf("Error updating the camera configuration for %d - %d : %v", temcaID, camera, dbErr)
		return
	}
	return
}
