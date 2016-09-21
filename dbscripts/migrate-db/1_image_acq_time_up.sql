ALTER TABLE roi_images CHANGE roi_included_images_id roi_image_id BIGINT NOT NULL AUTO_INCREMENT;
ALTER TABLE tem_camera_images ADD acquired_timestamp DATETIME NULL;
ALTER TABLE file_object_fs_replicas ADD location_url VARCHAR(1024);

CREATE INDEX tile_acquired_idx ON tem_camera_images (acquired_timestamp);

UPDATE file_object_fs_replicas SET location_url=CONCAT("http://sc1-jrc:81/proxy/bparc/", jfs_key) where jfs_key != '';
