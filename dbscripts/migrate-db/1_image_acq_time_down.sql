ALTER TABLE roi_images CHANGE roi_image_id roi_included_images_id BIGINT NOT NULL AUTO_INCREMENT;
ALTER TABLE tem_camera_images DROP acquired_timestamp;
ALTER TABLE file_object_fs_replicas DROP location_url;
