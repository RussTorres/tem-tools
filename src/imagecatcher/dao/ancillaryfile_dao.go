package dao

import (
	"imagecatcher/logger"
	"imagecatcher/models"
)

// CreateAncillaryFileType creates an ancillary file type if such type does not exist yet.
// If the type is already present it returns the existing record.
func CreateAncillaryFileType(fileType string, session DbSession) (int64, error) {
	sqlQuery := `
		select ancillary_file_type_id from ancillary_file_types where name = ?
	`
	rows, err := session.selectQuery(sqlQuery, fileType)
	defer closeRS(rows)
	if err != nil {
		return 0, err
	}
	if rows.Next() {
		var fileTypeID int64
		err = rows.Scan(&fileTypeID)
		if err != nil {
			logger.Errorf("Error extracting data from ancillary_file_type result set: %s", err)
		}
		return fileTypeID, err
	}
	logger.Debugf("Create ancillary file type '%s'", fileType)
	insertQuery := `
		insert into ancillary_file_types (name) values (?)
	`
	return insert(session, insertQuery, fileType)
}

// CreateAncillaryFile creates an ancillary file with the given type for the mosaic if the file does not already exist.
// If the file is already present it returns the existing record.
func CreateAncillaryFile(imageMosaic *models.TemImageMosaic, fname string, fileTypeID int64, session DbSession) (*models.FileObject, error) {
	sqlQuery := `
		select fo.file_object_fs_replica_id, fo.file_object_id
		from ancillary_files af
		join file_object_fs_replicas fo on fo.file_object_id = af.file_object_id
		where af.tem_camera_image_mosaic_id = ?
		and fo.path = ?
		and af.ancillary_file_type_id = ?
	`
	rows, err := session.selectQuery(sqlQuery, imageMosaic.ImageMosaicID, fileTypeID, fname)
	defer closeRS(rows)
	if err != nil {
		return nil, err
	}
	var f models.FileObject
	if rows.Next() {
		err = rows.Scan(&f.FileFSID, &f.FileObjectID)
		if err != nil {
			logger.Errorf("Error extracting data from ancillary_files result set: %s", err)
		}
		return &f, err
	}
	f.Path = fname
	if err = insertFileObj(&f, session); err != nil {
		return nil, err
	}
	if err = insertFileFS(&f, session); err != nil {
		return nil, err
	}
	logger.Debugf("Create ancillary file '%d'", f.FileObjectID)
	insertQuery := `
		insert into ancillary_files 
		(tem_camera_image_mosaic_id, file_object_id, ancillary_file_type_id) 
		values
		(?, ?, ?)
	`
	if _, err = insert(session, insertQuery, imageMosaic.ImageMosaicID, f.FileObjectID, fileTypeID); err != nil {
		return nil, err
	}
	return &f, nil
}
