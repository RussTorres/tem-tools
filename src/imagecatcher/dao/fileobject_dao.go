package dao

import (
	"time"

	"imagecatcher/models"
)

// UpdateFileObj - update FileObject entity
func UpdateFileObj(fileObj *models.FileObject, session DbSession) error {
	updateFSQuery := `
		update file_object_fs_replicas set
		path = ?,
		jfs_path = ?,
		jfs_key = ?,
		location_url = ?,
		file_exists_yn = ?
		where file_object_id = ?
	`
	if _, err := update(session, updateFSQuery, fileObj.Path, fileObj.JfsPath, fileObj.JfsKey, fileObj.LocationURL, fileObj.JfsKey != "", fileObj.FileObjectID); err != nil {
		return err
	}
	updateFOQuery := `
		update file_objects set
		sha1_checksum = ?,
		checksum_date = ?
		where file_object_id = ?
	`
	_, err := update(session, updateFOQuery, fileObj.Checksum, time.Now(), fileObj.FileObjectID)
	return err
}

func insertFileObj(fileObj *models.FileObject, session DbSession) error {
	if fileObj.Checksum != nil && len(fileObj.Checksum) > 0 && fileObj.ChecksumTimestamp == nil {
		fileObj.SetChecksumTimestamp(time.Now())
	}
	insertQuery := `
		insert into file_objects (sha1_checksum, checksum_date, creation_date) values (?, ?, ?)
	`
	id, err := insert(session, insertQuery, fileObj.Checksum, fileObj.ChecksumTimestamp, time.Now())
	fileObj.FileObjectID = id
	return err
}

func insertFileFS(fileObj *models.FileObject, session DbSession) error {
	insertQuery := `
		insert into file_object_fs_replicas
		(path, jfs_path, jfs_key, location_url, file_object_id, file_exists_yn, replica_created_date)
		values
		(?, ?, ?, ?, ?, ?, ?)
	`
	id, err := insert(session, insertQuery, fileObj.Path, fileObj.JfsPath, fileObj.JfsKey, fileObj.LocationURL, fileObj.FileObjectID, 0, time.Now())
	fileObj.FileFSID = id
	return err
}
