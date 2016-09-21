package dao

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagecatcher/logger"
	"imagecatcher/models"
)

// createMosaicROI creates a mosaic ROI based on the section name if such region doesn't exist yet
// If the ROI is already present it simply returns its ID.
func createMosaicROI(imageMosaic *models.TemImageMosaic, acqRoi *models.AcqROI, session DbSession) (int64, error) {
	// check if an entry exist in the image_mosaic_rois for the given section name
	sqlQuery := `
		select image_mosaic_roi_id from image_mosaic_rois where tem_camera_image_mosaic_id = ? and sample_section_name = ? and sample_section_id = ?
	`

	rows, err := session.selectQuery(sqlQuery, imageMosaic.ImageMosaicID, acqRoi.SectionName, acqRoi.NominalSection)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}

	if rows.Next() {
		var mosaicRoiID int64
		if err = rows.Scan(&mosaicRoiID); err != nil {
			return 0, fmt.Errorf("Error extracting data from image_mosaic_rois result set: %s", err)
		}
		return mosaicRoiID, nil
	}

	logger.Infof("Create Image Mosaic ROI for %v", acqRoi)
	insertQuery := `
		insert into image_mosaic_rois
		(tem_camera_image_mosaic_id, sample_section_id, sample_section_name)
		values
		(?, ?, ?)
	`
	return insert(session, insertQuery, imageMosaic.ImageMosaicID, acqRoi.NominalSection, acqRoi.SectionName)
}

// CreateROI creates a region of interest, which implies creating an entry both in image_mosaic_rois and in acq_time_rois.
// The model for the ROIs is not very intuitive - acq_time_rois holds the nominal section and it also references the image_mosaic_rois
// which holds a reference to a sample section which should be set before the acq_time_roi entry is created but that does not happen yet.
func CreateROI(im *models.TemImageMosaic, acqRoi *models.AcqROI, session DbSession) (int64, error) {
	var err error
	if acqRoi.MosaicRoiID, err = createMosaicROI(im, acqRoi, session); err != nil {
		return 0, err
	}

	// check if an acquisition ROI exists for the nominal section that references the mosaic ROI
	sqlQuery := `
		select acq_time_roi_id
		from acq_time_rois
		where tem_camera_image_mosaic_id = ? and image_mosaic_roi_id = ?
	`
	rows, err := session.selectQuery(sqlQuery, im.ImageMosaicID, acqRoi.MosaicRoiID)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}

	if rows.Next() {
		if err = rows.Scan(&acqRoi.AcqRoiID); err != nil {
			return 0, fmt.Errorf("Error extracting data from acq_time_rois result set: %s", err)
		}
		return acqRoi.MosaicRoiID, err
	}
	logger.Infof("Create Acq ROI for section '%v'", acqRoi)
	insertQuery := `
		insert into acq_time_rois
		(tem_camera_image_mosaic_id, image_mosaic_roi_id, acq_region_name, nominal_section_number) 
		values
		(?, ?, ?, ?)
	`
	if acqRoi.AcqRoiID, err = insert(session, insertQuery, im.ImageMosaicID, acqRoi.MosaicRoiID, acqRoi.RegionName, acqRoi.NominalSection); err != nil {
		return 0, err
	}
	return acqRoi.MosaicRoiID, nil
}

// CreateTileROI maps a tile to a mosaic ROI
func CreateTileROI(tm *models.TemImageMosaic, temImageID int64, acqRoi *models.AcqROI, roiState string, session DbSession) (int64, error) {
	sqlQuery := `
		select ri.roi_image_id
		from roi_images ri
		join tem_camera_images ti on ti.tem_camera_image_id = ri.tem_camera_image_id
		join image_mosaic_rois mr on mr.image_mosaic_roi_id = ri.image_mosaic_roi_id 
		where ti.tem_camera_image_mosaic_id = ?
		and ti.tem_camera_image_id = ?
		and mr.image_mosaic_roi_id = ?
	`
	rows, err := session.selectQuery(sqlQuery, tm.ImageMosaicID, temImageID, acqRoi.MosaicRoiID)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}

	if rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("Error extracting data from roi_images result set: %s", err)
		}
		return id, nil
	}
	var roiCurrentState string
	if roiState == "" {
		roiCurrentState = models.TileCreateState
	} else {
		roiCurrentState = roiState
	}
	insertQuery := `
		insert into roi_images
		(tem_camera_image_id, image_mosaic_roi_id, current_state)
		values
		(?, ?, ?)
	`
	return insert(session, insertQuery, temImageID, acqRoi.MosaicRoiID, roiCurrentState)
}

// CountPrevAcqContainingSection count all previous acquisition that contain the given section
func CountPrevAcqContainingSection(sampleName string, section int64, t time.Time, session DbSession) (c int, err error) {
	sqlQuery := `
		select
			count(acqf.uid)
		from ini_files_denorm acqf
		join tem_camera_image_mosaics tm on tm.ini_file_denorm_id = acqf.ini_file_denorm_id
		join acq_time_rois roi on roi.tem_camera_image_mosaic_id = tm.tem_camera_image_mosaic_id
		where acqf.when_acquired < ?
		and acqf.sample_name = ?
		and roi.nominal_section_number = ?
	`
	rows, err := session.selectQuery(sqlQuery, t, sampleName, section)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}
	if rows.Next() {
		if err = rows.Scan(&c); err != nil {
			return 0, fmt.Errorf("Error extracting count from CountPrevAcqContainingSection result set: %v", err)
		}
		return c, nil
	}
	return 0, nil
}

// RetrieveROIsBySections retrieves the region of interests for the given session
func RetrieveROIsBySections(imageMosaic *models.TemImageMosaic, sections []string, session DbSession) (map[string]*models.AcqROI, error) {
	rois := make(map[string]*models.AcqROI)
	if len(sections) == 0 {
		return rois, nil
	}
	sectionPlaceHolders := make([]string, len(sections))
	queryArgs := make([]interface{}, len(sections)+1)
	queryArgs[0] = imageMosaic.ImageMosaicID
	for i, s := range sections {
		sectionPlaceHolders[i] = "?"
		queryArgs[1+i] = s
	}

	var sqlQueryBuffer bytes.Buffer
	sqlQueryBuffer.WriteString(`
		select imr.image_mosaic_roi_id, aroi.acq_time_roi_id, aroi.acq_region_name, aroi.nominal_section_number, imr.sample_section_name
		from image_mosaic_rois imr
		join acq_time_rois aroi on aroi.image_mosaic_roi_id = imr.image_mosaic_roi_id
		where imr.tem_camera_image_mosaic_id = ?
		and imr.sample_section_name in
	`)
	sqlQueryBuffer.WriteString("(")
	sqlQueryBuffer.WriteString(strings.Join(sectionPlaceHolders, ","))
	sqlQueryBuffer.WriteString(")")

	rows, err := session.selectQuery(sqlQueryBuffer.String(), queryArgs...)
	if err != nil {
		return rois, err
	}
	defer closeRS(rows)
	var errList []string
	for rows.Next() {
		roi := &models.AcqROI{}
		err = rows.Scan(&roi.MosaicRoiID,
			&roi.AcqRoiID,
			&roi.RegionName,
			&roi.NominalSection,
			&roi.SectionName,
		)
		if err != nil {
			logger.Errorf("Error extracting data from image_mosaic_rois result set: %s", err)
			errList = append(errList, fmt.Sprintf("%s", err))
			continue
		}
		rois[roi.SectionName] = roi
	}
	if len(errList) > 0 {
		err = errors.New(strings.Join(errList, "\n"))
	}
	return rois, err
}

// RetrieveTileROIsByMosaicColAndRow rettrieve all ROIs of a tile identified by mosaic, row, col
func RetrieveTileROIsByMosaicColAndRow(mosaicID int64, col, row int, session DbSession) ([]*models.AcqROI, error) {
	sqlQuery := `
		select distinct
			roi.acq_time_roi_id,
			roi.image_mosaic_roi_id,
			roi.nominal_section_number
		from roi_images troi
		join tem_camera_images ti on troi.tem_camera_image_id = ti.tem_camera_image_id
		join tem_camera_image_mosaics tm on tm.tem_camera_image_mosaic_id = ti.tem_camera_image_mosaic_id
		join image_mosaic_rois mroi on mroi.image_mosaic_roi_id = troi.image_mosaic_roi_id and mroi.tem_camera_image_mosaic_id = tm.tem_camera_image_mosaic_id
		join acq_time_rois roi on roi.image_mosaic_roi_id = mroi.image_mosaic_roi_id
		where tm.tem_camera_image_mosaic_id = ?
		and ti.mosaic_col = ?
		and ti.mosaic_row = ?
	`
	rows, err := session.selectQuery(sqlQuery, mosaicID, col, row)
	defer closeRS(rows)

	tileROIs := make([]*models.AcqROI, 0, 10)
	if err != nil {
		return tileROIs, err
	}
	for rows.Next() {
		roi := &models.AcqROI{}
		err = rows.Scan(&roi.AcqRoiID,
			&roi.MosaicRoiID,
			&roi.NominalSection,
		)
		if err != nil {
			return tileROIs, err
		}
		tileROIs = append(tileROIs, roi)
	}
	return tileROIs, nil
}

// UpdateROIState updates ROI state
func UpdateROIState(temImageID int64, state string, session DbSession) (int64, error) {
	updateQuery := `
		update roi_images set
		current_state = ?
		where tem_camera_image_id = ?
	`
	return update(session, updateQuery, state, temImageID)
}
