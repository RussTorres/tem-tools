package dao

import (
	"bytes"
	"database/sql"
	"fmt"

	"imagecatcher/logger"
	"imagecatcher/models"
)

// RetrieveAllCalibrations - retrieves all calibrations
func RetrieveAllCalibrations(pagination models.Page, session DbSession) ([]*models.Calibration, error) {
	queryArgs := make([]interface{}, 0, 10)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select
			calibration_id,
			calibration_name,
			date_generated,
			json_config,
			notes
		from calibrations
	`)
	if pagination.StartRecordIndex > 0 || pagination.NRecords > 0 {
		var offset int64
		var length int32 = maxRows
		if pagination.StartRecordIndex > 0 {
			offset = pagination.StartRecordIndex
		}
		if pagination.NRecords > 0 {
			if pagination.NRecords > maxRows {
				length = maxRows
			} else {
				length = pagination.NRecords
			}
		}
		sqlQueryBuffer.WriteString("limit ? offset ? ")
		queryArgs = append(queryArgs, length, offset)
	} else {
		sqlQueryBuffer.WriteString("limit ? ")
		queryArgs = append(queryArgs, maxRows)
	}

	rows, err := session.selectQuery(sqlQueryBuffer.String(), queryArgs...)
	defer closeRS(rows)

	if err != nil {
		return nil, err
	}

	var calibrations []*models.Calibration
	var calibrationDate NullableTime
	var jsonBlob, notes sql.NullString
	var dbValErr error
	for rows.Next() {
		c := &models.Calibration{
			TemcaID: -1,
			Camera:  -1,
		}
		err = rows.Scan(&c.ID,
			&c.Name,
			&calibrationDate,
			&jsonBlob,
			&notes)
		if err != nil {
			dbValErr = fmt.Errorf("Error extracting data from calibrations result set: %s", err)
			logger.Error(dbValErr)
		}
		if calibrationDate.Valid {
			c.Generated = calibrationDate.Time
		}
		if jsonBlob.Valid {
			c.JSONContent = jsonBlob.String
		}
		if notes.Valid {
			c.Notes = notes.String
		}
		calibrations = append(calibrations, c)
	}
	return calibrations, dbValErr
}

// RetrieveCalibrationByName - retrieves the calibration by name
func RetrieveCalibrationByName(name string, session DbSession) ([]*models.Calibration, error) {
	sqlQuery := `
		select
			calibration_id,
			calibration_name,
			date_generated,
			json_config,
			notes
		from calibrations
		where calibration_name = ?
		order by date_generated desc
	`
	rows, err := session.selectQuery(sqlQuery, name)
	defer closeRS(rows)

	if err != nil {
		return nil, err
	}

	var calibrations []*models.Calibration
	var calibrationDate NullableTime
	var jsonBlob, notes sql.NullString
	var dbValErr error
	for rows.Next() {
		c := &models.Calibration{
			TemcaID: -1,
			Camera:  -1,
		}
		err = rows.Scan(&c.ID,
			&c.Name,
			&calibrationDate,
			&jsonBlob,
			&notes)
		if err != nil {
			dbValErr = fmt.Errorf("Error extracting data from calibrations result set: %s", err)
			logger.Error(dbValErr)
		}
		if calibrationDate.Valid {
			c.Generated = calibrationDate.Time
		}
		if jsonBlob.Valid {
			c.JSONContent = jsonBlob.String
		}
		if notes.Valid {
			c.Notes = notes.String
		}
		calibrations = append(calibrations, c)
	}
	return calibrations, dbValErr
}

// CreateCalibration creates a new calibration entry
func CreateCalibration(c *models.Calibration, session DbSession) error {
	insertQuery := `
	insert into calibrations (calibration_name, date_generated, json_config, notes)
	values
	(?, ?, ?, ?)
	`
	id, err := insert(session, insertQuery, c.Name, c.Generated, c.JSONContent, c.Notes)
	if err == nil {
		c.ID = id
	}
	return err
}

// UpdateCalibration updates an existing calibration entry
func UpdateCalibration(c *models.Calibration, session DbSession) error {
	updateQuery := `
	update calibrations set
	calibration_name = ?,
	date_generated = ?,
	json_config = ?,
	notes = ?
	where calibration_id = ?
	`
	_, err := update(session, updateQuery, c.Name, c.Generated, c.JSONContent, c.Notes, c.ID)
	return err
}

// CreateOrUpdateCalibration creates a calibration if the specified calibration name
// does not exist yet or updates it if it founds one entry with the specified name
func CreateOrUpdateCalibration(c *models.Calibration, session DbSession) error {
	existingCalibrations, err := RetrieveCalibrationByName(c.Name, session)
	if err != nil {
		return fmt.Errorf("Error retrieving any existing calibration entry by name %s: %v", c.Name, err)
	}
	if len(existingCalibrations) > 1 {
		return fmt.Errorf("Expected to retrieve only one calibration for %s but instead it found %d", c.Name, len(existingCalibrations))
	}
	if len(existingCalibrations) == 0 {
		return CreateCalibration(c, session)
	}
	c.ID = existingCalibrations[0].ID
	return UpdateCalibration(c, session)
}
