package dao

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/satori/go.uuid"
	"strings"

	"imagecatcher/logger"
	"imagecatcher/models"
)

// RetrieveTemImage - retrieves tile metadata
func RetrieveTemImage(tileID int64, session DbSession) (*models.TemImageROI, error) {
	// I join with the mosaic table to guarantee that the image is for the specified acquisition
	// If we start cleaning up the database before each run then the tile ids may overlap
	sqlQuery := `
		select
			ti.tem_camera_image_id,
			ti.tem_camera_image_mosaic_id,
			ti.tem_camera_configuration_id,
			ti.file_object_id,
			ti.frame_number,
			ti.mosaic_col,
			ti.mosaic_row,
			ti.acquired_timestamp,
			tc.tem_camera_number,
			fo.sha1_checksum,
			fr.jfs_path,
			fr.jfs_key,
			fr.location_url,
			acqf.uid,
			acqf.when_acquired,
			acqf.when_completed,
			acqf.sample_name,
			acqf.project_name,
			acqf.project_owner,
			acqf.stack_name,
			acqf.mosaic_type,
			troi.current_state,
			roi.acq_time_roi_id,
			roi.image_mosaic_roi_id,
			roi.nominal_section_number
		from tem_camera_images ti
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join file_objects fo on fo.file_object_id = ti.file_object_id
		join file_object_fs_replicas fr on fr.file_object_id = ti.file_object_id
		left outer join roi_images troi on troi.tem_camera_image_id = ti.tem_camera_image_id
		left outer join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id and mroi.tem_camera_image_mosaic_id = tm.tem_camera_image_mosaic_id
		left outer join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		left outer join tem_camera_configurations tc on tc.tem_camera_configuration_id = ti.tem_camera_configuration_id
		where ti.tem_camera_image_id = ?
	`
	rows, err := session.selectQuery(sqlQuery, tileID)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		temImage := &models.TemImageROI{}
		var cameraConfigurationID, cameraNumber sql.NullInt64
		var mosaicAcquiredTimestamp, mosaicCompletedTimestamp, tileAcquiredTimestamp NullableTime
		var jfsPath, jfsKey, tileFileURL sql.NullString
		var sampleName, projectName, projectOwner sql.NullString
		var stackName, mosaicType sql.NullString
		var tileState sql.NullString
		var acqRoiID, mosaicRoiID, tileSection sql.NullInt64
		err = rows.Scan(&temImage.ImageID,
			&temImage.ImageMosaic.ImageMosaicID,
			&cameraConfigurationID,
			&temImage.TileFile.FileObjectID,
			&temImage.Frame,
			&temImage.Col,
			&temImage.Row,
			&tileAcquiredTimestamp,
			&cameraNumber,
			&temImage.TileFile.Checksum,
			&jfsPath,
			&jfsKey,
			&tileFileURL,
			&temImage.ImageMosaic.AcqUID,
			&mosaicAcquiredTimestamp,
			&mosaicCompletedTimestamp,
			&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&tileState,
			&acqRoiID,
			&mosaicRoiID,
			&tileSection,
		)
		if err != nil {
			return temImage, fmt.Errorf("Error extracting data from tem_camera_images result set: %s", err)
		}
		if cameraConfigurationID.Valid {
			temImage.Configuration = &models.TemCameraConfiguration{
				ConfigurationID: cameraConfigurationID.Int64,
				Camera:          int(cameraNumber.Int64),
			}
		}
		temImage.ImageMosaic.Acquired = mosaicAcquiredTimestamp.Time
		if mosaicCompletedTimestamp.Valid {
			temImage.ImageMosaic.SetCompletedTimestamp(mosaicCompletedTimestamp.Time)
		}
		if tileAcquiredTimestamp.Valid {
			temImage.SetAcquiredTimestamp(tileAcquiredTimestamp.Time)
		}
		if jfsKey.Valid {
			temImage.TileFile.JfsKey = jfsKey.String
			temImage.TileFile.JfsPath = jfsPath.String
			temImage.TileFile.LocationURL = tileFileURL.String
		}
		if sampleName.Valid {
			temImage.ImageMosaic.SampleName = sampleName.String
		}
		if projectName.Valid {
			temImage.ImageMosaic.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			temImage.ImageMosaic.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			temImage.ImageMosaic.StackName = stackName.String
		}
		if mosaicType.Valid {
			temImage.ImageMosaic.MosaicType = mosaicType.String
		}
		if tileState.Valid {
			temImage.State = tileState.String
			temImage.Roi.AcqRoiID = acqRoiID.Int64
			temImage.Roi.MosaicRoiID = mosaicRoiID.Int64
			temImage.Roi.NominalSection = tileSection.Int64
		}
		return temImage, err
	}
	return nil, fmt.Errorf("No tile found for tile id %d", tileID)
}

// RetrieveTemImages retrieves the tiles that match the tileFilter parameters
func RetrieveTemImages(tileFilter *models.TileFilter, session DbSession) ([]*models.TemImageROI, error) {
	// I join with the mosaic table to guarantee that the image is for the specified acquisition
	// If we start cleaning up the database before each run then the tile ids may overlap
	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select
			ti.tem_camera_image_id,
			ti.tem_camera_image_mosaic_id,
			ti.tem_camera_configuration_id,
			ti.file_object_id,
			ti.frame_number,
			ti.mosaic_col,
			ti.mosaic_row,
			ti.acquired_timestamp,
			tc.tem_camera_number,
			fo.sha1_checksum,
			fr.jfs_path,
			fr.jfs_key,
			fr.location_url,
			acqf.uid,
			acqf.when_acquired,
			acqf.when_completed,
			acqf.sample_name,
			acqf.project_name,
			acqf.project_owner,
			acqf.stack_name,
			acqf.mosaic_type,
			troi.current_state,
			roi.acq_time_roi_id,
			roi.image_mosaic_roi_id,
			roi.nominal_section_number
		from tem_camera_images ti
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join file_objects fo on fo.file_object_id = ti.file_object_id
		join file_object_fs_replicas fr on fr.file_object_id = ti.file_object_id
		left outer join roi_images troi on troi.tem_camera_image_id = ti.tem_camera_image_id
		left outer join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id and mroi.tem_camera_image_mosaic_id = tm.tem_camera_image_mosaic_id
		left outer join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		left outer join tem_camera_configurations tc on tc.tem_camera_configuration_id = ti.tem_camera_configuration_id
	`)
	queryArgs := make([]interface{}, 0, 128)
	nextClause := "where "
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.AcqUID > 0,
		tileFilter.AcqUID,
		"acqf.uid = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.SampleName != "",
		tileFilter.SampleName,
		"acqf.sample_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.ProjectName != "",
		tileFilter.ProjectName,
		"acqf.project_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.ProjectOwner != "",
		tileFilter.ProjectOwner,
		"acqf.project_owner=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.StackName != "",
		tileFilter.StackName,
		"acqf.stack_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.MosaicType != "",
		tileFilter.MosaicType,
		"acqf.mosaic_type=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.Col >= 0,
		tileFilter.Col,
		"ti.mosaic_col = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.Row >= 0,
		tileFilter.Row,
		"ti.mosaic_row = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.IncludeOnlyPersisted(),
		nil,
		"fr.jfs_key is not null and fr.jfs_key != ''",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.CameraConfig.Camera >= 0,
		tileFilter.CameraConfig.Camera,
		"tc.tem_camera_number = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.State != "",
		tileFilter.State,
		"troi.current_state = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		tileFilter.ImageMosaicID > 0,
		tileFilter.ImageMosaicID,
		"tm.tem_camera_image_mosaic_id = ?",
		queryArgs,
		nextClause)
	if tileFilter.TileAcquiredInterval.From != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			tileFilter.TileAcquiredInterval.From,
			"ti.acquired_timestamp >= ?",
			queryArgs,
			nextClause)
	}
	if tileFilter.TileAcquiredInterval.To != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			tileFilter.TileAcquiredInterval.To,
			"ti.acquired_timestamp < ?",
			queryArgs,
			nextClause)
	}
	if tileFilter.FrameType != models.All {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			tileFilter.FrameType == models.Stable,
			nil,
			"ti.frame_number = -1",
			queryArgs,
			nextClause)
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			tileFilter.FrameType == models.Drift,
			nil,
			"ti.frame_number != -1",
			queryArgs,
			nextClause)
	}
	sqlQueryBuffer.WriteString("order by ti.acquired_timestamp ")
	if tileFilter.Pagination.StartRecordIndex > 0 || tileFilter.Pagination.NRecords > 0 {
		var offset int64
		var length int32 = maxRows
		if tileFilter.Pagination.StartRecordIndex > 0 {
			offset = tileFilter.Pagination.StartRecordIndex
		}
		if tileFilter.Pagination.NRecords > 0 && tileFilter.Pagination.NRecords < maxRows {
			length = tileFilter.Pagination.NRecords
		}
		sqlQueryBuffer.WriteString("limit ? offset ? ")
		queryArgs = append(queryArgs, length, offset)
	} else {
		sqlQueryBuffer.WriteString("limit ? ")
		queryArgs = append(queryArgs, maxRows)
	}
	rows, err := session.selectQuery(sqlQueryBuffer.String(), queryArgs...)
	defer closeRS(rows)

	temImages := make([]*models.TemImageROI, 0, 1024)
	if err != nil {
		return temImages, err
	}

	var rsExtractErr error
	var mosaicAcquiredTimestamp, mosaicCompletedTimestamp, tileAcquiredTimestamp NullableTime
	var cameraConfigurationID, cameraNumber sql.NullInt64
	var jfsPath, jfsKey, tileFileURL sql.NullString
	var sampleName, projectName, projectOwner sql.NullString
	var stackName, mosaicType sql.NullString
	var tileState sql.NullString
	var acqRoiID, mosaicRoiID, tileSection sql.NullInt64
	for rows.Next() {
		temImage := &models.TemImageROI{}
		err = rows.Scan(&temImage.ImageID,
			&temImage.ImageMosaic.ImageMosaicID,
			&cameraConfigurationID,
			&temImage.TileFile.FileObjectID,
			&temImage.Frame,
			&temImage.Col,
			&temImage.Row,
			&tileAcquiredTimestamp,
			&cameraNumber,
			&temImage.TileFile.Checksum,
			&jfsPath,
			&jfsKey,
			&tileFileURL,
			&temImage.ImageMosaic.AcqUID,
			&mosaicAcquiredTimestamp,
			&mosaicCompletedTimestamp,
			&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&tileState,
			&acqRoiID,
			&mosaicRoiID,
			&tileSection,
		)
		if err != nil {
			logger.Errorf("Error extracting data from tem_camera_images result set: %s", err)
			rsExtractErr = err
			continue
		}
		if cameraConfigurationID.Valid {
			temImage.Configuration = &models.TemCameraConfiguration{
				ConfigurationID: cameraConfigurationID.Int64,
				Camera:          int(cameraNumber.Int64),
			}
		}
		temImage.ImageMosaic.Acquired = mosaicAcquiredTimestamp.Time
		if mosaicCompletedTimestamp.Valid {
			temImage.ImageMosaic.SetCompletedTimestamp(mosaicCompletedTimestamp.Time)
		}
		if tileAcquiredTimestamp.Valid {
			temImage.SetAcquiredTimestamp(tileAcquiredTimestamp.Time)
		}
		if jfsKey.Valid {
			temImage.TileFile.JfsKey = jfsKey.String
			temImage.TileFile.JfsPath = jfsPath.String
			temImage.TileFile.LocationURL = tileFileURL.String
		}
		if sampleName.Valid {
			temImage.ImageMosaic.SampleName = sampleName.String
		}
		if projectName.Valid {
			temImage.ImageMosaic.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			temImage.ImageMosaic.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			temImage.ImageMosaic.StackName = stackName.String
		}
		if mosaicType.Valid {
			temImage.ImageMosaic.MosaicType = mosaicType.String
		}
		if tileState.Valid {
			temImage.State = tileState.String
			temImage.Roi.AcqRoiID = acqRoiID.Int64
			temImage.Roi.MosaicRoiID = mosaicRoiID.Int64
			temImage.Roi.NominalSection = tileSection.Int64
		}
		temImages = append(temImages, temImage)
	}
	return temImages, rsExtractErr
}

// CountTilesByStatus retrieves the number of tiles by acquisition/section/state as
// a map of counts indexed by acquisition id, section number and tile state.
// If the 'includeDrifts' flag is set to true it includes
// drifted frames in the returned resultset otherwise it returns only stable tiles (having frame = -1).
// If 'onlyWithPersistedFiles' is false it also includes tiles that don't have persisted images yet.
// Note: The method doesn't count by sample yet and it remains to see whether this is needed or not.
func CountTilesByStatus(filter *models.TileFilter, session DbSession) (map[uint64]map[int64]map[string]int, error) {
	queryArgs := make([]interface{}, 0, 128)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select
			acqf.uid,
			roi.nominal_section_number,
			troi.current_state,
			count(1)
		from tem_camera_images ti
		join tem_camera_configurations tc on tc.tem_camera_configuration_id = ti.tem_camera_configuration_id
		join file_object_fs_replicas tf on tf.file_object_id = ti.file_object_id
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join roi_images troi on troi.tem_camera_image_id = ti.tem_camera_image_id
		join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id
		join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
	`)
	nextClause := "where "
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.IncludeOnlyPersisted(),
		nil,
		"tf.jfs_key is not null and tf.jfs_key != ''",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID > 0,
		filter.AcqUID,
		"acqf.uid=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.SampleName != "",
		filter.SampleName,
		"acqf.sample_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.ProjectName != "",
		filter.ProjectName,
		"acqf.project_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.ProjectOwner != "",
		filter.ProjectOwner,
		"acqf.project_owner=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.StackName != "",
		filter.StackName,
		"acqf.stack_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.MosaicType != "",
		filter.MosaicType,
		"acqf.mosaic_type=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.NominalSection >= 0,
		filter.NominalSection,
		"roi.nominal_section_number = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.NominalSection < 0,
		nil,
		"roi.nominal_section_number is not NULL",
		queryArgs,
		nextClause)
	if filter.FrameType != models.All {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			filter.FrameType == models.Stable,
			nil,
			"ti.frame_number = -1",
			queryArgs,
			nextClause)
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			filter.FrameType == models.Drift,
			nil,
			"ti.frame_number != -1",
			queryArgs,
			nextClause)
	}
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		true,
		nil,
		"ti.tem_camera_configuration_id is not NULL",
		queryArgs,
		nextClause)
	sqlQueryBuffer.WriteString(`
		group by
			acqf.uid,
			roi.nominal_section_number,
			troi.current_state
	`)
	sqlQueryBuffer.WriteString(`
		order by
			acqf.uid,
			roi.nominal_section_number,
			troi.current_state
	`)
	rows, err := session.selectQuery(sqlQueryBuffer.String(), queryArgs...)
	defer closeRS(rows)

	countsByAcqSectionStatus := make(map[uint64]map[int64]map[string]int)
	if err != nil {
		return countsByAcqSectionStatus, err
	}
	var lastAcqIDCounts = struct {
		acqID           uint64
		countsBySection map[int64]map[string]int
	}{0, make(map[int64]map[string]int)}
	var lastSectionCounts = struct {
		section       int64
		countsByState map[string]int
	}{-1, make(map[string]int)}
	var lastStateCounts = struct {
		state string
		count int
	}{"", 0}
	var rsExtractErr error
	for rows.Next() {
		var currentAcqID uint64
		var currentSection int64
		var currentState string
		var count int
		err = rows.Scan(&currentAcqID,
			&currentSection,
			&currentState,
			&count,
		)
		if err != nil {
			logger.Errorf("Error extracting data from tile spec result set: %s", err)
			rsExtractErr = err
			continue
		}
		if currentAcqID != lastAcqIDCounts.acqID {
			if lastAcqIDCounts.acqID != 0 {
				countsByAcqSectionStatus[lastAcqIDCounts.acqID] = lastAcqIDCounts.countsBySection
				lastAcqIDCounts.countsBySection[lastSectionCounts.section] = lastSectionCounts.countsByState
				lastSectionCounts.countsByState[lastStateCounts.state] = lastStateCounts.count
			}
			lastAcqIDCounts = struct {
				acqID           uint64
				countsBySection map[int64]map[string]int
			}{currentAcqID, make(map[int64]map[string]int)}
			lastSectionCounts = struct {
				section       int64
				countsByState map[string]int
			}{-1, make(map[string]int)}
			lastStateCounts = struct {
				state string
				count int
			}{"", 0}
		}
		if currentSection != lastSectionCounts.section {
			if lastSectionCounts.section >= 0 {
				lastAcqIDCounts.countsBySection[lastSectionCounts.section] = lastSectionCounts.countsByState
				lastSectionCounts.countsByState[lastStateCounts.state] = lastStateCounts.count
			}
			lastSectionCounts = struct {
				section       int64
				countsByState map[string]int
			}{currentSection, make(map[string]int)}
			lastStateCounts = struct {
				state string
				count int
			}{"", 0}
		}
		if currentState != lastStateCounts.state {
			if lastStateCounts.state != "" {
				lastSectionCounts.countsByState[lastStateCounts.state] = lastStateCounts.count
			}
			lastStateCounts.state = currentState
			lastStateCounts.count = count
		}
	}

	if lastStateCounts.state != "" {
		lastSectionCounts.countsByState[lastStateCounts.state] = lastStateCounts.count
	}
	if lastSectionCounts.section >= 0 {
		lastAcqIDCounts.countsBySection[lastSectionCounts.section] = lastSectionCounts.countsByState
	}
	if lastAcqIDCounts.acqID != 0 {
		countsByAcqSectionStatus[lastAcqIDCounts.acqID] = lastAcqIDCounts.countsBySection
	}

	return countsByAcqSectionStatus, rsExtractErr
}

// UpdateTilesStatus update the status of the tiles that meet the given criteria and
// retrieve the tiles that have been updated.
// If valid 'AcqUID' and/or 'section' are provided it filters the tiles by the specified
// acquisition and/or section. If the 'IncludeDriftFrames' flag is set to true it includes
// drifted frames in the returned resultset otherwise it returns only stable images (having frame = -1).
// If 'IncludeNotPersisted' is true it also includes tiles that don't have persisted images yet.
func UpdateTilesStatus(filter *models.TileFilter, newState string, session DbSession) ([]*models.TileSpec, error) {
	txHash := uuid.NewV1().String()
	if nUpdates, err := updateTilesStatusUsingTx(filter, newState, txHash, session); err != nil || nUpdates == 0 {
		return []*models.TileSpec{}, err
	}
	return retrieveUpdatedTilesUsingTx(txHash, session)
}

// retrieveUpdatedTilesUsingTx retrieves tilespecs that have been updated in the specified transaction.
func retrieveUpdatedTilesUsingTx(txHash string, session DbSession) ([]*models.TileSpec, error) {
	queryArgs := make([]interface{}, 0, 128)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select
			acqf.sample_name,
			acqf.project_name,
			acqf.project_owner,
			acqf.stack_name,
			acqf.mosaic_type,
			acqf.uid,
			acqf.when_acquired,
			acqf.number_of_cameras,
			acqf.x_sm_step_pix,
			acqf.y_sm_step_pix,
			acqf.x_big_step_pix,
			acqf.y_big_step_pix,
			acqf.number_of_x_steps,
			acqf.number_of_y_steps,
			acqf.center_x_m,
			acqf.center_y_m,
			acqf.tem_y_m,
			acqf.pixels_per_um,
			acqf.tem_magnification,
			ti.tem_camera_image_id,
			ti.mosaic_col,
			ti.mosaic_row,
			ti.frame_number,
			troi.roi_image_id,
			troi.current_state,
			tc.tem_camera_number,
			tc.temca_id,
			tc.tem_camera_width,
			tc.tem_camera_height,
			tc.tem_camera_mask_url,
			tc.tem_camera_transformation_ref,
			roi.nominal_section_number,
			tf.jfs_path,
			tf.jfs_key,
			tf.location_url
		from tem_camera_images ti
		join tem_camera_configurations tc on tc.tem_camera_configuration_id = ti.tem_camera_configuration_id
		join file_object_fs_replicas tf on tf.file_object_id = ti.file_object_id
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join roi_images troi on troi.tem_camera_image_id = ti.tem_camera_image_id
		join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id
		join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		where troi.tx_hash = ?
	`)
	queryArgs = append(queryArgs, txHash)
	sqlQueryBuffer.WriteString(`
		order by
			acqf.when_acquired,
			roi.nominal_section_number,
			ti.mosaic_col,
			ti.mosaic_row
	`)
	rows, err := session.selectQuery(sqlQueryBuffer.String(), queryArgs...)
	defer closeRS(rows)

	tileSpecImages := make([]*models.TileSpec, 0, 1024)
	if err != nil {
		return tileSpecImages, err
	}

	var (
		rsExtractErr                          error
		acquired                              NullableTime
		sampleName, projectName, projectOwner sql.NullString
		stackName, mosaicType                 sql.NullString
		maskURL, transformationRef            sql.NullString
	)
	for rows.Next() {
		tileSpec := &models.TileSpec{}
		err = rows.Scan(&sampleName,
			&projectName,
			&projectOwner,
			&stackName,
			&mosaicType,
			&tileSpec.AcqUID,
			&acquired,
			&tileSpec.NumberOfCameras,
			&tileSpec.XSmallStepPix,
			&tileSpec.YSmallStepPix,
			&tileSpec.XBigStepPix,
			&tileSpec.YBigStepPix,
			&tileSpec.NXSteps,
			&tileSpec.NYSteps,
			&tileSpec.XCenter,
			&tileSpec.YCenter,
			&tileSpec.YTem,
			&tileSpec.PixPerUm,
			&tileSpec.Magnification,
			&tileSpec.TileImageID,
			&tileSpec.Col,
			&tileSpec.Row,
			&tileSpec.Frame,
			&tileSpec.RoiImageID,
			&tileSpec.State,
			&tileSpec.CameraConfig.Camera,
			&tileSpec.CameraConfig.TemcaID,
			&tileSpec.Width,
			&tileSpec.Height,
			&maskURL,
			&transformationRef,
			&tileSpec.NominalSection,
			&tileSpec.JFSPath,
			&tileSpec.JFSKey,
			&tileSpec.ImageURL,
		)
		if err != nil {
			logger.Errorf("Error extracting data from tile spec result set: %s", err)
			rsExtractErr = err
			continue
		}
		if sampleName.Valid {
			tileSpec.SampleName = sampleName.String
		}
		if projectName.Valid {
			tileSpec.ProjectName = projectName.String
		}
		if projectOwner.Valid {
			tileSpec.ProjectOwner = projectOwner.String
		}
		if stackName.Valid {
			tileSpec.StackName = stackName.String
		}
		if mosaicType.Valid {
			tileSpec.MosaicType = mosaicType.String
		}
		if maskURL.Valid {
			tileSpec.CameraConfig.MaskURL = maskURL.String
		}
		if transformationRef.Valid {
			tileSpec.CameraConfig.TransformationRef = transformationRef.String
		}
		tileSpec.Acquired = acquired.Time
		tileSpecImages = append(tileSpecImages, tileSpec)
	}
	return tileSpecImages, rsExtractErr
}

// updateTilesStatusUsingTx update the status of the tiles that meet the given criteria.
// If valid 'AcqUID' and/or 'section' are provided it filters the tiles by the specified
// acquisition and/or section. If the 'IncludeDriftFrames' flag is set to true it includes
// drifted frames in the returned resultset otherwise it returns only stable images (having frame = -1).
// If 'IncludeNotPersisted' is true it also includes tiles that don't have persisted images yet.
func updateTilesStatusUsingTx(filter *models.TileFilter, newState, txHash string, session DbSession) (int64, error) {
	queryArgs := make([]interface{}, 0, 128)
	queryArgs = append(queryArgs, newState, txHash)

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select troi.roi_image_id
		from roi_images troi
		join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id
		join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = mroi.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join tem_camera_images ti on troi.tem_camera_image_id = ti.tem_camera_image_id
		join tem_camera_configurations tc on tc.tem_camera_configuration_id = ti.tem_camera_configuration_id
		join file_object_fs_replicas tf on tf.file_object_id = ti.file_object_id
		where troi.current_state = ?
	`)
	queryArgs = append(queryArgs, filter.State)
	nextClause := "and "
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.NominalSection >= 0,
		filter.NominalSection,
		"roi.nominal_section_number = ?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.NominalSection < 0,
		nil,
		"roi.nominal_section_number is not NULL",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID > 0,
		filter.AcqUID,
		"acqf.uid=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.SampleName != "",
		filter.SampleName,
		"acqf.sample_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.ProjectName != "",
		filter.ProjectName,
		"acqf.project_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.ProjectOwner != "",
		filter.ProjectOwner,
		"acqf.project_owner=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.StackName != "",
		filter.StackName,
		"acqf.stack_name=?",
		queryArgs,
		nextClause)
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.AcqUID == 0 && filter.MosaicType != "",
		filter.MosaicType,
		"acqf.mosaic_type=?",
		queryArgs,
		nextClause)
	if filter.FrameType != models.All {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			filter.FrameType == models.Stable,
			nil,
			"ti.frame_number = -1",
			queryArgs,
			nextClause)
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			filter.FrameType == models.Drift,
			nil,
			"ti.frame_number != -1",
			queryArgs,
			nextClause)
	}
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		true,
		nil,
		"ti.tem_camera_configuration_id is not NULL",
		queryArgs,
		nextClause)
	if filter.TileAcquiredInterval.From != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			filter.TileAcquiredInterval.From,
			"ti.acquired_timestamp >= ?",
			queryArgs,
			nextClause)
	}
	if filter.TileAcquiredInterval.To != nil {
		queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
			true,
			filter.TileAcquiredInterval.To,
			"ti.acquired_timestamp < ?",
			queryArgs,
			nextClause)
	}
	queryArgs, nextClause = addFlagQueryCond(&sqlQueryBuffer,
		filter.IncludeOnlyPersisted(),
		nil,
		"tf.jfs_key is not NULL and tf.jfs_key != ''",
		queryArgs,
		nextClause)
	sqlQueryBuffer.WriteString(`
		order by
			acqf.when_acquired,
			roi.nominal_section_number,
			ti.mosaic_col,
			ti.mosaic_row
	`)
	if filter.Pagination.StartRecordIndex > 0 || filter.Pagination.NRecords > 0 {
		var offset int64
		var length int32 = maxRows
		if filter.Pagination.StartRecordIndex > 0 {
			offset = filter.Pagination.StartRecordIndex
		}
		if filter.Pagination.NRecords > 0 && filter.Pagination.NRecords < maxRows {
			length = filter.Pagination.NRecords
		}
		sqlQueryBuffer.WriteString("limit ? offset ? ")
		queryArgs = append(queryArgs, length, offset)
	}

	var updateQueryBuffer bytes.Buffer
	updateQueryBuffer.WriteString(`
		update roi_images troi
		set current_state = ?, tx_hash = ?
		where  troi.roi_image_id in 
	`)
	updateQueryBuffer.WriteString("(select roi_image_id from (")
	updateQueryBuffer.WriteString(sqlQueryBuffer.String())
	updateQueryBuffer.WriteString(") tmp) ")

	return update(session, updateQueryBuffer.String(), queryArgs...)
}

// CreateTemImage creates a tile image that is not associated with any camera or file.
func CreateTemImage(im *models.TemImageMosaic, tileCol int, tileRow int, imageFrame int, session DbSession) (*models.TemImage, error) {
	sqlQuery := `
		select 
			ti.tem_camera_image_id,
			ti.tem_camera_image_mosaic_id,
			ti.tem_camera_configuration_id,
			ti.file_object_id,
			ti.frame_number,
			ti.mosaic_col,
			ti.mosaic_row
		from tem_camera_images ti
		where ti.tem_camera_image_mosaic_id = ?
		and ti.mosaic_col = ?
		and ti.mosaic_row = ?
		and ti.frame_number = ?
	`
	rows, err := session.selectQuery(sqlQuery, im.ImageMosaicID, tileCol, tileRow, imageFrame)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}

	var temImage models.TemImage
	if rows.Next() {
		var cameraConfigurationID sql.NullInt64
		err = rows.Scan(&temImage.ImageID,
			&temImage.ImageMosaic.ImageMosaicID,
			&cameraConfigurationID,
			&temImage.TileFile.FileObjectID,
			&temImage.Frame,
			&temImage.Col,
			&temImage.Row)
		if err != nil {
			return &temImage, fmt.Errorf("Error extracting data from tem_camera_images result set: %s", err)
		}
		if cameraConfigurationID.Valid {
			temImage.Configuration = &models.TemCameraConfiguration{ConfigurationID: cameraConfigurationID.Int64}
		}
		temImage.ImageMosaic = *im
	} else {
		temImage.ImageMosaic = *im
		temImage.Col = tileCol
		temImage.Row = tileRow
		temImage.Frame = imageFrame

		if err = insertFileObj(&temImage.TileFile, session); err != nil {
			return &temImage, err
		}
		if err = insertFileFS(&temImage.TileFile, session); err != nil {
			return &temImage, err
		}
		insertQuery := `
			insert into tem_camera_images
			(
			mosaic_col,
			mosaic_row,
			frame_number,
			tem_camera_image_mosaic_id,
			file_object_id
			)
			values
			(?, ?, ?, ?, ?)
		`
		temImage.ImageID, err = insert(session, insertQuery,
			temImage.Col,
			temImage.Row,
			temImage.Frame,
			temImage.ImageMosaic.ImageMosaicID,
			temImage.TileFile.FileObjectID)
	}
	return &temImage, err
}

// UpdateTemImageCamera - updates tile metadata
func UpdateTemImageCamera(temImage *models.TemImage, session DbSession) error {
	updateQuery := `
		update tem_camera_images set tem_camera_configuration_id = ? where tem_camera_image_id = ?
	`
	_, err := update(session, updateQuery, temImage.Configuration.ConfigurationID, temImage.ImageID)
	return err
}

// UpdateTemImage updates tile image metadata - currently it only updates the acquired timestamp.
func UpdateTemImage(temImage *models.TemImage, session DbSession) error {
	updateQuery := `
		update tem_camera_images set
		acquired_timestamp = ?
		where tem_camera_image_id = ?
	`
	_, err := update(session, updateQuery, temImage.AcquiredTimestamp, temImage.ImageID)
	return err
}

// UpdateTemImageROIState updates the state for all provided TEM Images to the given state value.
func UpdateTemImageROIState(tiles []*models.TileSpec, state string, session DbSession) (int64, error) {
	queryArgs := make([]interface{}, 1)
	var updateQueryBuffer bytes.Buffer
	updateQueryBuffer.WriteString(`
		update roi_images troi
		join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id
		join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		join tem_camera_images ti on troi.tem_camera_image_id = ti.tem_camera_image_id
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join ini_files_denorm acqf on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		set troi.current_state = ?
		where
	`)
	queryArgs[0] = state

	var conditions []string
	var tileConditionBuffer bytes.Buffer
	for _, tile := range tiles {
		op := ""
		if tile.RoiImageID > 0 {
			tileConditionBuffer.WriteString(op)
			tileConditionBuffer.WriteString("troi.roi_image_id = ? ")
			queryArgs = append(queryArgs, tile.RoiImageID)
			op = "and "
		}
		if tile.AcqUID > 0 {
			tileConditionBuffer.WriteString(op)
			tileConditionBuffer.WriteString("acqf.uid = ? ")
			queryArgs = append(queryArgs, tile.AcqUID)
			op = "and "
		}
		if tile.NominalSection >= 0 {
			tileConditionBuffer.WriteString(op)
			tileConditionBuffer.WriteString("roi.nominal_section_number = ? ")
			queryArgs = append(queryArgs, tile.NominalSection)
			op = "and "
		}
		if tile.Col >= 0 {
			tileConditionBuffer.WriteString(op)
			tileConditionBuffer.WriteString("ti.mosaic_col = ? ")
			queryArgs = append(queryArgs, tile.Col)
			op = "and "
		}
		if tile.Row >= 0 {
			tileConditionBuffer.WriteString(op)
			tileConditionBuffer.WriteString("ti.mosaic_row = ? ")
			queryArgs = append(queryArgs, tile.Row)
			op = "and "
		}
		if op != "" {
			conditions = append(conditions, "("+tileConditionBuffer.String()+")")
		}
		tileConditionBuffer.Reset()
	}
	if len(conditions) == 0 {
		return 0, fmt.Errorf("A condition must be specified for updating the ROI state for %v", tiles)
	}
	updateQueryBuffer.WriteString(strings.Join(conditions, " OR "))
	return update(session, updateQueryBuffer.String(), queryArgs...)
}
