package dao

import (
	"bytes"
	"database/sql"
	"fmt"
	"net"
	"time"

	"imagecatcher/logger"
	"imagecatcher/models"
)

// CreateAcqLog creates the acquisition log if it doesn't exist, otherwise simply return the existing record.
func CreateAcqLog(acq *models.Acquisition, session DbSession) error {
	acqfID, err := getAcqLog(acq, session)
	if err != nil {
		return fmt.Errorf("Error reading the INI acquisition log for '%d': %v", acq.AcqUID, err)
	}
	if acqfID != 0 {
		logger.Infof("Update INI Log for '%d'", acq.AcqUID)
		nUpdates, err := updateAcquisition(acq, session)
		if err != nil {
			return fmt.Errorf("Error updating the acquisition '%d': %v", acq.AcqUID, err)
		}
		logger.Infof("Number of updates for '%d' -> %d", acq.AcqUID, nUpdates)
		return nil
	}
	logger.Infof("Create INI Log for '%d'", acq.AcqUID)
	if acq.IniFileID, err = insertAcquisition(acq, session); err != nil {
		return fmt.Errorf("Error creating a new acquisition for '%d': %v", acq.AcqUID, err)
	}
	return nil
}

func getAcqLog(acq *models.Acquisition, session DbSession) (int64, error) {
	sqlQuery := `
		select
			ini_file_denorm_id,
			ini_file
		from ini_files_denorm where uid = ?
	`
	rows, err := session.selectQuery(sqlQuery, acq.AcqUID)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}
	if rows.Next() {
		err = rows.Scan(&acq.IniFileID, &acq.IniContent)
		if err != nil {
			return 0, fmt.Errorf("Error extracting data from INI file result set: %v", err)
		}
		return acq.IniFileID, nil
	}
	return 0, nil
}

func insertAcquisition(acq *models.Acquisition, session DbSession) (int64, error) {
	insertQuery := `
		insert into ini_files_denorm
		(
		uid,
		ini_file,
		when_acquired,
		when_inserted,
		when_completed,
		number_of_cameras,
		x_sm_step_pix,
		y_sm_step_pix,
		x_big_step_pix,
		y_big_step_pix,
		number_of_x_steps,
		number_of_y_steps,
		center_x_m,
		center_y_m,
		tem_y_m,
		pixels_per_um,
		tem_magnification,
		url,
		sample_name,
		project_name,
		project_owner,
		stack_name,
		mosaic_type,
		microscopist_name,
		roi_tiles_file,
		notes
		)
		values
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	return insert(session, insertQuery,
		acq.AcqUID,
		acq.IniContent,
		acq.Acquired,
		acq.Inserted,
		acq.Completed,
		acq.NumberOfCameras,
		acq.XSmallStepPix,
		acq.YSmallStepPix,
		acq.XBigStepPix,
		acq.YBigStepPix,
		acq.NXSteps,
		acq.NYSteps,
		acq.XCenter,
		acq.YCenter,
		acq.YTem,
		acq.PixPerUm,
		acq.Magnification,
		acq.URL,
		acq.SampleName,
		acq.ProjectName,
		acq.ProjectOwner,
		acq.StackName,
		acq.MosaicType,
		acq.MicroscopistName,
		acq.RoiTilesFile,
		acq.Notes)
}

func updateAcquisition(acq *models.Acquisition, session DbSession) (int64, error) {
	updateQuery := `
		update ini_files_denorm set
		ini_file = ?,
		when_acquired = ?,
		when_inserted = ?,
		when_completed = ?,
		number_of_cameras = ?,
		x_sm_step_pix = ?,
		y_sm_step_pix = ?,
		x_big_step_pix = ?,
		y_big_step_pix = ?,
		number_of_x_steps = ?,
		number_of_y_steps = ?,
		center_x_m = ?,
		center_y_m = ?,
		tem_y_m = ?,
		pixels_per_um = ?,
		tem_magnification = ?,
		url = ?,
		sample_name = ?,
		project_name = ?,
		project_owner = ?,
		stack_name = ?,
		mosaic_type = ?,
		microscopist_name = ?,
		roi_tiles_file = ?,
		notes = ?
		where uid = ?
	`
	return update(session, updateQuery,
		acq.IniContent,
		acq.Acquired,
		acq.Inserted,
		acq.Completed,
		acq.NumberOfCameras,
		acq.XSmallStepPix,
		acq.YSmallStepPix,
		acq.XBigStepPix,
		acq.YBigStepPix,
		acq.NXSteps,
		acq.NYSteps,
		acq.XCenter,
		acq.YCenter,
		acq.YTem,
		acq.PixPerUm,
		acq.Magnification,
		acq.URL,
		acq.SampleName,
		acq.ProjectName,
		acq.ProjectOwner,
		acq.StackName,
		acq.MosaicType,
		acq.MicroscopistName,
		acq.RoiTilesFile,
		acq.Notes,
		acq.AcqUID,
	)
}

// EndAcquisition marks the acquisition as completed
func EndAcquisition(acqID uint64, session DbSession) error {
	updateAcqQuery := `
		update ini_files_denorm
		set when_completed = ?
		where uid = ?
	`
	_, err := update(session, updateAcqQuery, time.Now(), acqID)
	return err
}

// CreateImageMosaic creates the image mosaic entity for the given acquisition
func CreateImageMosaic(acq *models.Acquisition, session DbSession) (*models.TemImageMosaic, error) {
	imageMosaic, err := RetrieveImageMosaicByAcqUID(acq.AcqUID, session)
	if err != nil {
		return nil, err
	}
	// retrieve the temca
	var temcaID interface{}
	var foundTemcaID, temID int64
	if temID, err = retrieveTemID(acq.TemIPAddress, acq.Acquired, session); err != nil {
		return nil, fmt.Errorf("Error retrieving TEM for %s: %v", acq.TemIPAddress, err)
	}
	if temID != 0 {
		if foundTemcaID, err = retrieveTemca(temID, acq.Acquired, session); err != nil {
			return nil, fmt.Errorf("Error retrieving TEMCA for %d:%s: %v", temID, acq.TemIPAddress, err)
		}
	}
	if foundTemcaID != 0 {
		temcaID = foundTemcaID
	} else {
		logger.Infof("No TEMCA found for %d:%s", temID, acq.TemIPAddress)
	}
	if imageMosaic == nil {
		insertQuery := `
			insert into tem_camera_image_mosaics
			(
			ini_file_denorm_id,
			target_num_cols,
			target_num_rows,
			temca_id
			) 
			values
			(?, ?, ?, ?)
		`
		imageMosaicID, err := insert(session, insertQuery, acq.IniFileID, acq.NTargetCols, acq.NTargetRows, temcaID)
		if err != nil {
			return imageMosaic, err
		}
		imageMosaic = &models.TemImageMosaic{}
		imageMosaic.ImageMosaicID = imageMosaicID
		imageMosaic.IniFileID = acq.IniFileID
		imageMosaic.AcqUID = acq.AcqUID
		imageMosaic.NumCols = acq.NTargetCols
		imageMosaic.NumRows = acq.NTargetRows
		imageMosaic.Temca.TemcaID = foundTemcaID
		return imageMosaic, err
	}
	updateQuery := `
		update tem_camera_image_mosaics set
		target_num_cols = ?,
		target_num_rows = ?,
		temca_id = ?
		where tem_camera_image_mosaic_id = ?
	`
	nUpdates, err := update(session, updateQuery, acq.NTargetCols, acq.NTargetRows, temcaID, imageMosaic.ImageMosaicID)
	if err != nil {
		return imageMosaic, fmt.Errorf("Error updating %d mosaic: %v", acq.AcqUID, err)
	}
	logger.Infof("Number of tem_camera_image_mosaics updated: %d", nUpdates)
	return imageMosaic, nil
}

const selectAcqQuery = `
	select
		acqf.ini_file_denorm_id,
		tcm.tem_camera_image_mosaic_id,
		acqf.when_acquired,
		acqf.when_inserted,
		acqf.when_completed,
		acqf.uid as acqf_uid,
		acqf.number_of_cameras,
		acqf.x_sm_step_pix, acqf.y_sm_step_pix,
		acqf.x_big_step_pix, acqf.y_big_step_pix,
		acqf.number_of_x_steps, acqf.number_of_y_steps,
		acqf.center_x_m, acqf.center_y_m, acqf.tem_y_m,
		acqf.pixels_per_um, acqf.tem_magnification,
		acqf.url,
		acqf.sample_name,
		acqf.project_name,
		acqf.project_owner,
		acqf.stack_name ,
		acqf.mosaic_type,
		acqf.microscopist_name,
		acqf.roi_tiles_file,
		acqf.notes
	from ini_files_denorm acqf
	join tem_camera_image_mosaics tcm on acqf.ini_file_denorm_id = tcm.ini_file_denorm_id
`

// RetrieveAcquisition retrieves an acquisition by its UID
func RetrieveAcquisition(acqID uint64, session DbSession) (*models.Acquisition, error) {
	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(selectAcqQuery)
	sqlQueryBuffer.WriteString("where acqf.uid=?")

	rows, err := session.selectQuery(sqlQueryBuffer.String(), acqID)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}
	var (
		acq                                   *models.Acquisition
		acquired, inserted, completedTime     NullableTime
		sampleName, projectName, projectOwner sql.NullString
		stackName, mosaicType                 sql.NullString
	)

	if rows.Next() {
		acq = &models.Acquisition{}

		err = rows.Scan(&acq.IniFileID,
			&acq.ImageMosaicID,
			&acquired,
			&inserted,
			&completedTime,
			&acq.AcqUID,
			&acq.NumberOfCameras,
			&acq.XSmallStepPix, &acq.YSmallStepPix,
			&acq.XBigStepPix, &acq.YBigStepPix,
			&acq.NXSteps, &acq.NYSteps,
			&acq.XCenter, &acq.YCenter, &acq.YTem,
			&acq.PixPerUm, &acq.Magnification,
			&acq.URL,
			&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&acq.MicroscopistName,
			&acq.RoiTilesFile,
			&acq.Notes,
		)
		if err != nil {
			return acq, fmt.Errorf("Error extracting data from acquisition result for %d: %v", acqID, err)
		}
		acq.Acquired = acquired.Time
		acq.Inserted = inserted.Time
		if completedTime.Valid {
			acq.SetCompletedTimestamp(completedTime.Time)
		}
		if sampleName.Valid {
			acq.SampleName = sampleName.String
		}
		if projectName.Valid {
			acq.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			acq.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			acq.StackName = stackName.String
		}
		if mosaicType.Valid {
			acq.MosaicType = mosaicType.String
		}
	}
	return acq, nil
}

// RetrieveAcquisitions retrieves a list of Acquisitions entities; if acqID is set it filters the result set by acquisition UID
// The method could also filter acquisitions for which there exists at least one tile in a state specified by 'RequiredStateForAtLeastOneTile'
// or if it has all tiles in the state specified by 'RequiredStateForAllTiles'.
func RetrieveAcquisitions(acqFilter *models.AcquisitionFilter, session DbSession) ([]*models.Acquisition, error) {
	queryArgs := make([]interface{}, 0, 10)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(selectAcqQuery)
	otherStatesCountQuery := `
		select sum(c1)
		from (
			select acqf1.uid, troi1.current_state, count(1) as c1
			from roi_images troi1
			join image_mosaic_rois mroi1 on mroi1.image_mosaic_roi_id = troi1.image_mosaic_roi_id
			join tem_camera_image_mosaics tm1 on tm1.tem_camera_image_mosaic_id = mroi1.tem_camera_image_mosaic_id
			join ini_files_denorm acqf1 on tm1.ini_file_denorm_id = acqf1.ini_file_denorm_id
			group by acqf1.uid, troi1.current_state
		) state_counts1
		where state_counts1.uid = acqf.uid and state_counts1.current_state != ?
	`
	specifiedStateCountQuery := `
		select c2
		from (
			select acqf2.uid, troi2.current_state, count(1) as c2
			from roi_images troi2
			join image_mosaic_rois mroi2 on mroi2.image_mosaic_roi_id = troi2.image_mosaic_roi_id
			join tem_camera_image_mosaics tm2 on tm2.tem_camera_image_mosaic_id = mroi2.tem_camera_image_mosaic_id
			join ini_files_denorm acqf2 on tm2.ini_file_denorm_id = acqf2.ini_file_denorm_id
			group by acqf2.uid, troi2.current_state
		) state_counts2
		where state_counts2.uid = acqf.uid and state_counts2.current_state = ?
	`
	nextClause := "where "
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.AcqUID > 0,
		acqFilter.AcqUID,
		"acqf.uid=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.SampleName != "",
		acqFilter.SampleName,
		"acqf.sample_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.ProjectName != "",
		acqFilter.ProjectName,
		"acqf.project_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.ProjectOwner != "",
		acqFilter.ProjectOwner,
		"acqf.project_owner=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.StackName != "",
		acqFilter.StackName,
		"acqf.stack_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.MosaicType != "",
		acqFilter.MosaicType,
		"acqf.mosaic_type=?",
		queryArgs,
		nextClause)
	if acqFilter.RequiredStateForAllTiles != "" {
		// for checking whether it has all in the specified state
		// it checks if there's any tile in any other state and if
		// the specified state has at least one tile
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true, // we know the condition is true already
			acqFilter.RequiredStateForAllTiles,
			"("+otherStatesCountQuery+") is null",
			queryArgs,
			nextClause)
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true, // we know the condition is true already
			acqFilter.RequiredStateForAllTiles,
			"("+specifiedStateCountQuery+") >= 1",
			queryArgs,
			nextClause)
	}
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		acqFilter.RequiredStateForAtLeastOneTile != "",
		acqFilter.RequiredStateForAtLeastOneTile,
		"("+specifiedStateCountQuery+") >= 1",
		queryArgs,
		nextClause)
	if acqFilter.AcquiredInterval.From != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			acqFilter.AcquiredInterval.From,
			"acqf.when_acquired>=?",
			queryArgs,
			nextClause)
	}
	if acqFilter.AcquiredInterval.To != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			acqFilter.AcquiredInterval.To,
			"acqf.when_acquired<?",
			queryArgs,
			nextClause)
	}
	sqlQueryBuffer.WriteString(`
		order by
			acqf.uid
	`)
	if acqFilter.Pagination.StartRecordIndex > 0 || acqFilter.Pagination.NRecords > 0 {
		var offset int64
		var length int32 = maxRows
		if acqFilter.Pagination.StartRecordIndex > 0 {
			offset = acqFilter.Pagination.StartRecordIndex
		}
		if acqFilter.Pagination.NRecords > 0 {
			if acqFilter.Pagination.NRecords > maxRows {
				length = maxRows
			} else {
				length = acqFilter.Pagination.NRecords
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
	var (
		acquisitions                          []*models.Acquisition
		acquired, inserted, completedTime     NullableTime
		sampleName, projectName, projectOwner sql.NullString
		stackName, mosaicType                 sql.NullString
	)
	for rows.Next() {
		acq := &models.Acquisition{}

		err = rows.Scan(&acq.IniFileID,
			&acq.ImageMosaicID,
			&acquired,
			&inserted,
			&completedTime,
			&acq.AcqUID,
			&acq.NumberOfCameras,
			&acq.XSmallStepPix, &acq.YSmallStepPix,
			&acq.XBigStepPix, &acq.YBigStepPix,
			&acq.NXSteps, &acq.NYSteps,
			&acq.XCenter, &acq.YCenter, &acq.YTem,
			&acq.PixPerUm, &acq.Magnification,
			&acq.URL,
			&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&acq.MicroscopistName,
			&acq.RoiTilesFile,
			&acq.Notes,
		)
		if err != nil {
			return acquisitions, fmt.Errorf("Error extracting data from acquisition result: %s", err)
		}
		acq.Acquired = acquired.Time
		acq.Inserted = inserted.Time
		if completedTime.Valid {
			acq.SetCompletedTimestamp(completedTime.Time)
		}
		if sampleName.Valid {
			acq.SampleName = sampleName.String
		}
		if projectName.Valid {
			acq.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			acq.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			acq.StackName = stackName.String
		}
		if mosaicType.Valid {
			acq.MosaicType = mosaicType.String
		}
		acquisitions = append(acquisitions, acq)
	}
	return acquisitions, nil
}

// RetrieveImageMosaicByAcqUID retrieves the ImageMosaic entity for the specified acquisition UID
func RetrieveImageMosaicByAcqUID(acqID uint64, session DbSession) (*models.TemImageMosaic, error) {
	sqlQuery := `
		select 
			tcm.tem_camera_image_mosaic_id,
			tcm.temca_id,
			tcm.target_num_cols,
			tcm.target_num_rows,
			acqf.ini_file_denorm_id,
			acqf.uid, 
			acqf.sample_name,
			acqf.project_name,
			acqf.project_owner,
			acqf.stack_name ,
			acqf.mosaic_type,
			temcas.tem_id,
			temcas.tem_scintillator_id,
			temcas.temca_type_id
		from tem_camera_image_mosaics tcm
		join ini_files_denorm acqf on acqf.ini_file_denorm_id = tcm.ini_file_denorm_id
		left outer join temcas on temcas.temca_id = tcm.temca_id
		where acqf.uid = ?
	`
	rows, err := session.selectQuery(sqlQuery, acqID)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		var temcaID, temID, temcaTypeID, temScintillatorID sql.NullInt64
		var sampleName, projectName, projectOwner sql.NullString
		var stackName, mosaicType sql.NullString
		imageMosaic := &models.TemImageMosaic{}
		err = rows.Scan(&imageMosaic.ImageMosaicID,
			&temcaID,
			&imageMosaic.NumCols,
			&imageMosaic.NumRows,
			&imageMosaic.IniFileID,
			&imageMosaic.AcqUID,
			&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&temID,
			&temScintillatorID,
			&temcaTypeID)
		if err != nil {
			return imageMosaic, fmt.Errorf("Error extracting data from tem_camera_image_mosaic result set for %d: %s", acqID, err)
		}
		if sampleName.Valid {
			imageMosaic.SampleName = sampleName.String
		}
		if projectName.Valid {
			imageMosaic.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			imageMosaic.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			imageMosaic.StackName = stackName.String
		}
		if mosaicType.Valid {
			imageMosaic.MosaicType = mosaicType.String
		}
		if temcaID.Valid {
			imageMosaic.Temca.TemcaID = temcaID.Int64
		}
		if temID.Valid {
			imageMosaic.Temca.TemID = temID.Int64
		}
		if temcaTypeID.Valid {
			imageMosaic.Temca.TemScintillatorID = temScintillatorID.Int64
		}
		if temScintillatorID.Valid {
			imageMosaic.Temca.TemcaTypeID = temcaTypeID.Int64
		}
		return imageMosaic, nil
	}
	return nil, nil
}

func retrieveTemID(ip string, acquireTime time.Time, session DbSession) (int64, error) {
	var temIP []byte
	var parsedIP net.IP
	if ip == "" {
		parsedIP = net.ParseIP("0.0.0.0").To4()
	} else {
		parsedIP = net.ParseIP(ip).To4()
	}
	temIP = []byte{parsedIP[0], parsedIP[1], parsedIP[2], parsedIP[3]}
	sqlQuery := `
		select tem_pc_ip_address_id 
		from tem_pc_ip_addresses
		where ip_address = ? 
		and start_date <= ?
		and (end_date is null or end_date > ?)
	`
	rows, err := session.selectQuery(sqlQuery, temIP, acquireTime, acquireTime)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}

	if rows.Next() {
		var temID int64
		err = rows.Scan(&temID)
		if err != nil {
			return temID, fmt.Errorf("Error extracting data from tem_pc_ip_addresses result set: %s", err)
		}
		return temID, nil
	}
	return 0, nil
}

func retrieveTemca(temID int64, acquireTime time.Time, session DbSession) (int64, error) {
	sqlQuery := `
		select temca_id 
		from temcas
		where tem_id = ? 
		and start_date <= ?
		and (end_date is null or end_date > ?)
	`
	rows, err := session.selectQuery(sqlQuery, temID, acquireTime, acquireTime)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}

	if rows.Next() {
		var temcaID int64
		err = rows.Scan(&temcaID)
		if err != nil {
			return temcaID, fmt.Errorf("Error extracting data from temcas result set: %s", err)
		}
		return temcaID, nil
	}
	return 0, nil
}

// RetrieveDistinctAcquisitionProjects retrieves a list of distinct Acquisitions projects.
func RetrieveDistinctAcquisitionProjects(filter *models.ProjectFilter, session DbSession) ([]*models.Project, error) {
	queryArgs := make([]interface{}, 0, 10)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select distinct
			acqf.project_name,
			acqf.project_owner
		from ini_files_denorm acqf
	`)

	nextClause := "where "
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.SampleName != "",
		filter.SampleName,
		"acqf.sample_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.ProjectName != "",
		filter.ProjectName,
		"acqf.project_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.ProjectOwner != "",
		filter.ProjectOwner,
		"acqf.project_owner=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.StackName != "",
		filter.StackName,
		"acqf.stack_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.MosaicType != "",
		filter.MosaicType,
		"acqf.mosaic_type=?",
		queryArgs,
		nextClause)
	if filter.DataAcquiredInterval.From != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			filter.DataAcquiredInterval.From,
			"acqf.when_acquired>=?",
			queryArgs,
			nextClause)
	}
	if filter.DataAcquiredInterval.To != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			filter.DataAcquiredInterval.To,
			"acqf.when_acquired<?",
			queryArgs,
			nextClause)
	}
	sqlQueryBuffer.WriteString(`
		order by
			acqf.project_name
	`)
	if filter.Pagination.StartRecordIndex > 0 || filter.Pagination.NRecords > 0 {
		var offset int64
		var length int32 = maxRows
		if filter.Pagination.StartRecordIndex > 0 {
			offset = filter.Pagination.StartRecordIndex
		}
		if filter.Pagination.NRecords > 0 {
			if filter.Pagination.NRecords > maxRows {
				length = maxRows
			} else {
				length = filter.Pagination.NRecords
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
	var (
		projects                  []*models.Project
		projectName, projectOwner sql.NullString
	)
	for rows.Next() {
		proj := &models.Project{}

		err = rows.Scan(&projectName,
			&projectOwner,
		)
		if err != nil {
			return projects, fmt.Errorf("Error extracting data from projects result: %s", err)
		}
		if projectName.Valid {
			proj.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			proj.ProjectOwner = projectOwner.String
		}
		projects = append(projects, proj)
	}
	return projects, nil
}
