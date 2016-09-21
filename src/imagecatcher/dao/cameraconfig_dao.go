package dao

import (
	"bytes"
	"database/sql"
	"fmt"

	"imagecatcher/logger"
	"imagecatcher/models"
)

const selectCameraConfigsQuery = `
	select
		tcfg.tem_camera_configuration_id, tcfg.temca_id, tcfg.tem_camera_id, tcfg.tem_camera_number,
		tcfg.tem_camera_array_col, tcfg.tem_camera_array_row,
		tcfg.tem_camera_width, tcfg.tem_camera_height,
		tcfg.tem_camera_mask_url, tcfg.tem_camera_transformation_ref
	from tem_camera_configurations tcfg
`

// RetrieveCameraConfigurationsByTemcaID - retrieves the camera configuration
func RetrieveCameraConfigurationsByTemcaID(temcaID int64, session DbSession) (map[int]models.TemCameraConfiguration, error) {
	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(selectCameraConfigsQuery)
	sqlQueryBuffer.WriteString("where temca_id = ?")

	rows, err := session.selectQuery(sqlQueryBuffer.String(), temcaID)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}

	var dbValErr error
	var transformationRef, maskURL sql.NullString
	cameraConfigurations := make(map[int]models.TemCameraConfiguration)
	for rows.Next() {
		var cameraConfiguration models.TemCameraConfiguration
		err = rows.Scan(&cameraConfiguration.ConfigurationID,
			&cameraConfiguration.TemcaID,
			&cameraConfiguration.CameraID,
			&cameraConfiguration.Camera,
			&cameraConfiguration.CameraArrayCol,
			&cameraConfiguration.CameraArrayRow,
			&cameraConfiguration.Width,
			&cameraConfiguration.Height,
			&maskURL,
			&transformationRef,
		)
		if err != nil {
			dbValErr = fmt.Errorf("Error extracting data from tem_camera_configurations result set: %v", err)
			logger.Error(dbValErr)
		}
		if transformationRef.Valid {
			cameraConfiguration.TransformationRef = transformationRef.String
		}
		if maskURL.Valid {
			cameraConfiguration.MaskURL = maskURL.String
		}
		cameraConfigurations[cameraConfiguration.Camera] = cameraConfiguration
	}
	return cameraConfigurations, dbValErr
}

// RetrieveCameraConfigurationsByTemcaIDAndCameraNo - retrieves the camera configuration by temca and camera number
func RetrieveCameraConfigurationsByTemcaIDAndCameraNo(temcaID int64, cameraNumber int, session DbSession) ([]*models.TemCameraConfiguration, error) {
	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(selectCameraConfigsQuery)
	sqlQueryBuffer.WriteString("where temca_id = ? and tem_camera_number = ?")

	rows, err := session.selectQuery(sqlQueryBuffer.String(), temcaID, cameraNumber)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}

	var dbValErr error
	var transformationRef, maskURL sql.NullString
	var cameraConfigurations []*models.TemCameraConfiguration
	for rows.Next() {
		cameraConfiguration := &models.TemCameraConfiguration{}
		err = rows.Scan(&cameraConfiguration.ConfigurationID,
			&cameraConfiguration.TemcaID,
			&cameraConfiguration.CameraID,
			&cameraConfiguration.Camera,
			&cameraConfiguration.CameraArrayCol,
			&cameraConfiguration.CameraArrayRow,
			&cameraConfiguration.Width,
			&cameraConfiguration.Height,
			&maskURL,
			&transformationRef,
		)
		if err != nil {
			dbValErr = fmt.Errorf("Error extracting data from tem_camera_configurations result set: %v", err)
			logger.Error(dbValErr)
		}
		if transformationRef.Valid {
			cameraConfiguration.TransformationRef = transformationRef.String
		}
		if maskURL.Valid {
			cameraConfiguration.MaskURL = maskURL.String
		}
		cameraConfigurations = append(cameraConfigurations, cameraConfiguration)
	}
	return cameraConfigurations, dbValErr
}

// UpdateCameraConfiguration - retrieves the camera configuration by temca and camera number
func UpdateCameraConfiguration(camCfg *models.TemCameraConfiguration, session DbSession) error {
	updateQuery := `
		update tem_camera_configurations set
			tem_camera_id = ?,
			temca_id = ?,
			tem_camera_number = ?,
			tem_camera_array_col = ?,
			tem_camera_array_row = ?,
			tem_camera_width = ?,
			tem_camera_height = ?,
			tem_camera_mask_url = ?,
			tem_camera_transformation_ref = ?
		where tem_camera_configuration_id = ?
	`
	_, err := update(session, updateQuery,
		camCfg.CameraID,
		camCfg.TemcaID,
		camCfg.Camera,
		camCfg.CameraArrayCol,
		camCfg.CameraArrayRow,
		camCfg.Width,
		camCfg.Height,
		camCfg.MaskURL,
		camCfg.TransformationRef,
		camCfg.ConfigurationID,
	)
	return err
}
