CREATE TABLE daisy_manufacturers (
	daisy_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (daisy_manufacturer_id)
);

CREATE TABLE data_source_types (
	data_source_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(50),
	notes VARCHAR(255),
	PRIMARY KEY (data_source_type_id)
);

CREATE TABLE file_objects (
	file_object_id BIGINT NOT NULL AUTO_INCREMENT,
	sha1_checksum BINARY(20),
	checksum_date DATETIME,
	creation_date DATETIME,
	PRIMARY KEY (file_object_id)
);

CREATE TABLE people (
	person_id INTEGER NOT NULL AUTO_INCREMENT,
	username VARCHAR(16) NOT NULL,
	first_name VARCHAR(16),
	last_name VARCHAR(16),
	email VARCHAR(64),
	notes VARCHAR(255),
	PRIMARY KEY (person_id),
	UNIQUE (username)
);

CREATE TABLE calibrations (
	calibration_id INTEGER NOT NULL AUTO_INCREMENT,
	calibration_name VARCHAR(50) NOT NULL,
	date_generated DATETIME,
	json_config TEXT,
	notes VARCHAR(255),
	PRIMARY KEY (calibration_id)
);

CREATE TABLE grid_manufacturers (
	grid_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (grid_manufacturer_id)
);

CREATE TABLE species (
	species_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (species_id)
);

CREATE TABLE post_staining_methods (
	post_staining_method_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (post_staining_method_id)
);

CREATE TABLE ini_files_denorm (
	ini_file_denorm_id INTEGER NOT NULL AUTO_INCREMENT,
	ini_file TEXT,
	when_acquired DATETIME NOT NULL,
	when_inserted DATETIME NOT NULL,
	when_completed DATETIME,
	uid BIGINT NOT NULL,
	number_of_cameras INTEGER NOT NULL,
	x_sm_step_pix INTEGER,
	y_sm_step_pix INTEGER,
	x_big_step_pix INTEGER,
	y_big_step_pix INTEGER,
	number_of_x_steps INTEGER,
	number_of_y_steps INTEGER,
	center_x_m FLOAT,
	center_y_m FLOAT,
	tem_y_m FLOAT,
	slot_midpoint_x_m FLOAT,
	slot_midpoint_y_m FLOAT,
	slot_angle_deg FLOAT,
	pixels_per_um FLOAT,
	tem_magnification INTEGER,
	url VARCHAR(1024),
	sample_name VARCHAR(255),
	project_name varchar(80),
	project_owner varchar(80),
	stack_name varchar(80),
	mosaic_type varchar(32),
	microscopist_name VARCHAR(50),
	roi_tiles_file VARCHAR(1024),
	notes TEXT,
	PRIMARY KEY (ini_file_denorm_id)
);

CREATE INDEX ix_ini_files_denorm_sample_name ON ini_files_denorm (sample_name);
CREATE UNIQUE INDEX ix_ini_files_denorm_uid ON ini_files_denorm (uid);
CREATE INDEX ix_ini_files_denorm_when_acquired ON ini_files_denorm (when_acquired);

CREATE TABLE tem_scintillator_manufacturers (
	tem_scintillator_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_scintillator_manufacturer_id)
);

CREATE TABLE grid_photograph_sources (
	grid_photograph_source_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (grid_photograph_source_id)
);

CREATE TABLE magazines (
	magazine_id INTEGER NOT NULL AUTO_INCREMENT,
	visible_id INTEGER,
	notes TEXT,
	PRIMARY KEY (magazine_id)
);

CREATE TABLE thin_sectioning_methods (
	thin_sectioning_method_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (thin_sectioning_method_id)
);

CREATE TABLE cassette_manufacturers (
	cassette_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (cassette_manufacturer_id)
);

CREATE TABLE ancillary_file_types (
	ancillary_file_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255) NOT NULL,
	description VARCHAR(255),
	PRIMARY KEY (ancillary_file_type_id)
);

CREATE TABLE temca_types (
	temca_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255),
	number_of_cameras INTEGER,
	notes VARCHAR(255),
	PRIMARY KEY (temca_type_id)
);

CREATE TABLE terrace_trays (
	terrace_tray_id INTEGER NOT NULL AUTO_INCREMENT,
	visible_id INTEGER,
	notes TEXT,
	PRIMARY KEY (terrace_tray_id)
);

CREATE TABLE support_film_types (
	support_film_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (support_film_type_id)
);

CREATE TABLE tem_manufacturers (
	tem_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_manufacturer_id)
);

CREATE TABLE tem_camera_manufacturers (
	tem_camera_manufacturer_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_camera_manufacturer_id)
);

CREATE TABLE calibration_source_mosaic_groups (
	calibration_source_mosaic_group_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(30) NOT NULL,
	notes VARCHAR(255),
	PRIMARY KEY (calibration_source_mosaic_group_id),
	UNIQUE (name)
);

CREATE TABLE tem_scintillator_types (
	tem_scintillator_type_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_scintillator_manufacturer_id INTEGER,
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_scintillator_type_id),
	FOREIGN KEY(tem_scintillator_manufacturer_id) REFERENCES tem_scintillator_manufacturers (tem_scintillator_manufacturer_id)
);

CREATE TABLE path_prefixes (
	path_prefix_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(50) NOT NULL,
	path VARCHAR(255),
	data_source_type_id INTEGER,
	PRIMARY KEY (path_prefix_id),
	FOREIGN KEY(data_source_type_id) REFERENCES data_source_types (data_source_type_id)
);

CREATE TABLE grid_types (
	grid_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	grid_manufacturer_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (grid_type_id),
	FOREIGN KEY(grid_manufacturer_id) REFERENCES grid_manufacturers (grid_manufacturer_id)
);

CREATE TABLE tems (
	tem_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_manufacturer_id INTEGER NOT NULL,
	visible_id VARCHAR(16),
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_id),
	FOREIGN KEY(tem_manufacturer_id) REFERENCES tem_manufacturers (tem_manufacturer_id)
);

CREATE TABLE image_stacks (
	image_stack_id INTEGER NOT NULL AUTO_INCREMENT,
	name VARCHAR(255) NOT NULL,
	calibration_id INTEGER,
	description VARCHAR(255),
	PRIMARY KEY (image_stack_id),
	FOREIGN KEY(calibration_id) REFERENCES calibrations (calibration_id)
);

CREATE TABLE tomograms (
	tomogram_id INTEGER NOT NULL AUTO_INCREMENT,
	file_object_id BIGINT NOT NULL,
	notes TEXT,
	PRIMARY KEY (tomogram_id),
	FOREIGN KEY(file_object_id) REFERENCES file_objects (file_object_id)
);

CREATE TABLE daisy_types (
	daisy_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	daisy_manufacturer_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (daisy_type_id),
	FOREIGN KEY(daisy_manufacturer_id) REFERENCES daisy_manufacturers (daisy_manufacturer_id)
);

CREATE TABLE cassette_types (
	cassette_type_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	cassette_manufacturer_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (cassette_type_id),
	FOREIGN KEY(cassette_manufacturer_id) REFERENCES cassette_manufacturers (cassette_manufacturer_id)
);

CREATE TABLE tem_camera_types (
	tem_camera_type_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_camera_manufacturer_id INTEGER,
	name VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_camera_type_id),
	FOREIGN KEY(tem_camera_manufacturer_id) REFERENCES tem_camera_manufacturers (tem_camera_manufacturer_id)
);

CREATE TABLE grids (
	grid_id INTEGER NOT NULL AUTO_INCREMENT,
	visible_id INTEGER,
	grid_type_id INTEGER NOT NULL,
	grid_lot_number INTEGER,
	support_film_type_id INTEGER NOT NULL,
	support_film_coating_person_id INTEGER NOT NULL,
	post_staining_person_id INTEGER NOT NULL,
	post_staining_method_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (grid_id),
	FOREIGN KEY(grid_type_id) REFERENCES grid_types (grid_type_id),
	FOREIGN KEY(support_film_type_id) REFERENCES support_film_types (support_film_type_id),
	FOREIGN KEY(support_film_coating_person_id) REFERENCES people (person_id),
	FOREIGN KEY(post_staining_person_id) REFERENCES people (person_id),
	FOREIGN KEY(post_staining_method_id) REFERENCES post_staining_methods (post_staining_method_id)
);

CREATE TABLE tem_pc_ip_addresses (
	tem_pc_ip_address_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_id INTEGER NOT NULL,
	ip_address BINARY(4) NOT NULL,
	start_date DATETIME,
	end_date DATETIME,
	PRIMARY KEY (tem_pc_ip_address_id),
	FOREIGN KEY(tem_id) REFERENCES tems (tem_id)
);

CREATE TABLE file_object_fs_replicas (
	file_object_fs_replica_id BIGINT NOT NULL AUTO_INCREMENT,
	path_prefix_id INTEGER,
	path VARCHAR(255),
	relative_path BOOL,
	jfs_path VARCHAR(1024),
	jfs_key VARCHAR(64),
	location_url VARCHAR(1024),
	file_object_id BIGINT NOT NULL,
	derived_from_file_object_fs_replica_id BIGINT,
	file_exists_yn BOOL NOT NULL,
	last_verified DATETIME,
	replica_created_date DATETIME NOT NULL,
	PRIMARY KEY (file_object_fs_replica_id),
	FOREIGN KEY(path_prefix_id) REFERENCES path_prefixes (path_prefix_id),
	CHECK (relative_path IN (0,1)),
	FOREIGN KEY(file_object_id) REFERENCES file_objects (file_object_id),
	FOREIGN KEY(derived_from_file_object_fs_replica_id) REFERENCES file_object_fs_replicas (file_object_fs_replica_id),
	CHECK (file_exists_yn IN (0,1))
);
CREATE INDEX ix_file_object_fs_replicas_path ON file_object_fs_replicas (path);
CREATE INDEX ix_file_object_fs_replicas_jfs_key ON file_object_fs_replicas (jfs_key);

CREATE TABLE tem_cameras (
	tem_camera_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_camera_type_id INTEGER,
	visible_id VARCHAR(16),
	notes VARCHAR(255),
	PRIMARY KEY (tem_camera_id),
	FOREIGN KEY(tem_camera_type_id) REFERENCES tem_camera_types (tem_camera_type_id)
);

CREATE TABLE cassettes (
	cassette_id INTEGER NOT NULL AUTO_INCREMENT,
	visible_id TEXT,
	cassette_type_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (cassette_id),
	FOREIGN KEY(cassette_type_id) REFERENCES cassette_types (cassette_type_id)
);

CREATE TABLE daisies (
	daisy_id INTEGER NOT NULL AUTO_INCREMENT,
	visible_id INTEGER,
	daisy_type_id INTEGER NOT NULL,
	notes TEXT,
	PRIMARY KEY (daisy_id),
	FOREIGN KEY(daisy_type_id) REFERENCES daisy_types (daisy_type_id)
);

CREATE TABLE tem_scintillators (
	tem_scintillator_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_scintillator_type_id INTEGER,
	visible_id VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_scintillator_id),
	FOREIGN KEY(tem_scintillator_type_id) REFERENCES tem_scintillator_types (tem_scintillator_type_id)
);

CREATE TABLE blocks (
	block_id INTEGER NOT NULL AUTO_INCREMENT,
	name TEXT,
	description TEXT,
	maker_person_id INTEGER NOT NULL,
	tomogram_id INTEGER NOT NULL,
	PRIMARY KEY (block_id),
	FOREIGN KEY(maker_person_id) REFERENCES people (person_id),
	FOREIGN KEY(tomogram_id) REFERENCES tomograms (tomogram_id)
);

CREATE TABLE grid_photographs (
	grid_photograph_id INTEGER NOT NULL AUTO_INCREMENT,
	grid_id INTEGER NOT NULL,
	grid_photograph_blob BLOB,
	file_object_id BIGINT NOT NULL,
	grid_photograph_source_id INTEGER NOT NULL,
	when_acquired DATETIME,
	notes TEXT,
	PRIMARY KEY (grid_photograph_id),
	FOREIGN KEY(grid_id) REFERENCES grids (grid_id),
	FOREIGN KEY(file_object_id) REFERENCES file_objects (file_object_id),
	FOREIGN KEY(grid_photograph_source_id) REFERENCES grid_photograph_sources (grid_photograph_source_id)
);

CREATE TABLE grid_daisy_history (
	grid_daisy_history_id INTEGER NOT NULL AUTO_INCREMENT,
	start_date DATETIME,
	end_date DATETIME,
	loading_person_id INTEGER NOT NULL,
	unloading_person_id INTEGER NOT NULL,
	grid_id INTEGER NOT NULL,
	daisy_id INTEGER NOT NULL,
	daisy_level INTEGER,
	daisy_pocket INTEGER,
	notes TEXT,
	PRIMARY KEY (grid_daisy_history_id),
	FOREIGN KEY(loading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(unloading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(grid_id) REFERENCES grids (grid_id),
	FOREIGN KEY(daisy_id) REFERENCES daisies (daisy_id)
);

CREATE TABLE samples (
	sample_id INTEGER NOT NULL AUTO_INCREMENT,
	block_id INTEGER NOT NULL,
	species_id INTEGER NOT NULL,
	name TEXT,
	notes TEXT,
	PRIMARY KEY (sample_id),
	FOREIGN KEY(block_id) REFERENCES blocks (block_id),
	FOREIGN KEY(species_id) REFERENCES species (species_id)
);

CREATE TABLE temcas (
	temca_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_id INTEGER,
	tem_scintillator_id INTEGER,
	temca_type_id INTEGER,
	start_date DATETIME,
	end_date DATETIME,
	notes VARCHAR(255),
	PRIMARY KEY (temca_id),
	FOREIGN KEY(tem_id) REFERENCES tems (tem_id),
	FOREIGN KEY(tem_scintillator_id) REFERENCES tem_scintillators (tem_scintillator_id),
	FOREIGN KEY(temca_type_id) REFERENCES temca_types (temca_type_id)
);

CREATE TABLE cassette_magazine_history (
	cassette_magazine_history_id INTEGER NOT NULL AUTO_INCREMENT,
	start_date DATETIME,
	end_date DATETIME,
	loading_person_id INTEGER NOT NULL,
	unloading_person_id INTEGER NOT NULL,
	cassette_id INTEGER NOT NULL,
	magazine_id INTEGER NOT NULL,
	magazine_slot INTEGER,
	notes TEXT,
	PRIMARY KEY (cassette_magazine_history_id),
	FOREIGN KEY(loading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(unloading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(cassette_id) REFERENCES cassettes (cassette_id),
	FOREIGN KEY(magazine_id) REFERENCES magazines (magazine_id)
);

CREATE TABLE grid_cassette_history (
	grid_cassette_history_id INTEGER NOT NULL AUTO_INCREMENT,
	start_date DATETIME,
	end_date DATETIME,
	loading_person_id INTEGER NOT NULL,
	unloading_person_id INTEGER NOT NULL,
	grid_id INTEGER NOT NULL,
	cassette_id INTEGER NOT NULL,
	cassette_pocket_row INTEGER,
	cassette_pocket_col INTEGER,
	notes TEXT,
	PRIMARY KEY (grid_cassette_history_id),
	FOREIGN KEY(loading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(unloading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(grid_id) REFERENCES grids (grid_id),
	FOREIGN KEY(cassette_id) REFERENCES cassettes (cassette_id)
);

CREATE TABLE grid_terrace_tray_history (
	grid_terrace_tray_history_id INTEGER NOT NULL AUTO_INCREMENT,
	start_date DATETIME,
	end_date DATETIME,
	loading_person_id INTEGER NOT NULL,
	unloading_person_id INTEGER NOT NULL,
	grid_id INTEGER NOT NULL,
	terrace_tray_id INTEGER NOT NULL,
	terrace_tray_pocket INTEGER,
	notes TEXT,
	PRIMARY KEY (grid_terrace_tray_history_id),
	FOREIGN KEY(loading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(unloading_person_id) REFERENCES people (person_id),
	FOREIGN KEY(grid_id) REFERENCES grids (grid_id),
	FOREIGN KEY(terrace_tray_id) REFERENCES terrace_trays (terrace_tray_id)
);

CREATE TABLE thin_sections (
	thin_section_id INTEGER NOT NULL AUTO_INCREMENT,
	slice_number INTEGER,
	nominal_thickness_nm INTEGER,
	grid_id INTEGER NOT NULL,
	block_id INTEGER NOT NULL,
	sectioning_person_id INTEGER NOT NULL,
	thin_sectioning_method_id INTEGER NOT NULL,
	coming_in_yn INTEGER,
	knife_marks_yn INTEGER,
	chatter_yn INTEGER,
	support_film_wrinkles INTEGER,
	section_wrinkles INTEGER,
	spaghetti_yn INTEGER,
	split_yn INTEGER,
	destroyed_yn INTEGER,
	notes TEXT,
	PRIMARY KEY (thin_section_id),
	FOREIGN KEY(grid_id) REFERENCES grids (grid_id),
	FOREIGN KEY(block_id) REFERENCES blocks (block_id),
	FOREIGN KEY(sectioning_person_id) REFERENCES people (person_id),
	FOREIGN KEY(thin_sectioning_method_id) REFERENCES thin_sectioning_methods (thin_sectioning_method_id)
);

CREATE TABLE tem_camera_image_mosaics (
	tem_camera_image_mosaic_id INTEGER NOT NULL AUTO_INCREMENT,
	ini_file_denorm_id INTEGER NOT NULL,
	temca_id INTEGER,
	person_id INTEGER,
	target_num_cols INTEGER NOT NULL,
	target_num_rows INTEGER NOT NULL,
	PRIMARY KEY (tem_camera_image_mosaic_id),
	FOREIGN KEY(ini_file_denorm_id) REFERENCES ini_files_denorm (ini_file_denorm_id),
	FOREIGN KEY(temca_id) REFERENCES temcas (temca_id),
	FOREIGN KEY(person_id) REFERENCES people (person_id)
);

CREATE TABLE tem_camera_configurations (
	tem_camera_configuration_id INTEGER NOT NULL AUTO_INCREMENT,
	temca_id INTEGER NOT NULL,
	tem_camera_id INTEGER,
	tem_camera_number SMALLINT,
	tem_camera_array_col SMALLINT,
	tem_camera_array_row SMALLINT,
	tem_camera_width INTEGER,
	tem_camera_height INTEGER,
	tem_camera_mask_url VARCHAR(255),
	tem_camera_transformation_ref VARCHAR(255),
	notes VARCHAR(255),
	PRIMARY KEY (tem_camera_configuration_id),
	FOREIGN KEY(temca_id) REFERENCES temcas (temca_id),
	FOREIGN KEY(tem_camera_id) REFERENCES tem_cameras (tem_camera_id)
);

CREATE TABLE sample_sections (
	sample_section_id INTEGER NOT NULL AUTO_INCREMENT,
	thin_section_id INTEGER NOT NULL,
	sample_id INTEGER NOT NULL,
	nominal_thickness INTEGER,
	serial_section_number INTEGER,
	notes TEXT,
	PRIMARY KEY (sample_section_id),
	FOREIGN KEY(thin_section_id) REFERENCES thin_sections (thin_section_id),
	FOREIGN KEY(sample_id) REFERENCES samples (sample_id)
);

CREATE TABLE image_mosaic_rois (
	image_mosaic_roi_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_camera_image_mosaic_id INTEGER NOT NULL,
	sample_section_id INTEGER,
	sample_section_name VARCHAR(64),
	PRIMARY KEY (image_mosaic_roi_id),
	FOREIGN KEY(tem_camera_image_mosaic_id) REFERENCES tem_camera_image_mosaics (tem_camera_image_mosaic_id)
);
CREATE INDEX mosaic_roi_section_idx ON image_mosaic_rois (sample_section_id,sample_section_name);

CREATE TABLE ancillary_files (
	ancillary_file_id BIGINT NOT NULL AUTO_INCREMENT,
	tem_camera_image_mosaic_id INTEGER,
	file_object_id BIGINT,
	ancillary_file_type_id INTEGER,
	PRIMARY KEY (ancillary_file_id),
	FOREIGN KEY(tem_camera_image_mosaic_id) REFERENCES tem_camera_image_mosaics (tem_camera_image_mosaic_id),
	FOREIGN KEY(file_object_id) REFERENCES file_objects (file_object_id),
	FOREIGN KEY(ancillary_file_type_id) REFERENCES ancillary_file_types (ancillary_file_type_id)
);

CREATE TABLE tem_camera_images (
	tem_camera_image_id BIGINT NOT NULL AUTO_INCREMENT,
	tem_camera_image_mosaic_id INTEGER NOT NULL,
	tem_camera_configuration_id INTEGER,
	file_object_id BIGINT,
	frame_number INTEGER,
	mosaic_col INTEGER NOT NULL,
	mosaic_row INTEGER NOT NULL,
	u8HistOffset FLOAT,
	u8HistScale FLOAT,
	acquired_timestamp DATETIME NULL,
	PRIMARY KEY (tem_camera_image_id),
	FOREIGN KEY(tem_camera_image_mosaic_id) REFERENCES tem_camera_image_mosaics (tem_camera_image_mosaic_id),
	FOREIGN KEY(tem_camera_configuration_id) REFERENCES tem_camera_configurations (tem_camera_configuration_id),
	FOREIGN KEY(file_object_id) REFERENCES file_objects (file_object_id)
);
CREATE INDEX tile_col_row_frame_idx ON tem_camera_images (mosaic_col,mosaic_row,frame_number);
CREATE INDEX tile_acquired_idx ON tem_camera_images (acquired_timestamp);

CREATE TABLE calibration_source_mosaics (
	calibration_source_mosaic_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_camera_image_mosaic_id INTEGER NOT NULL,
	calibration_source_mosaic_group_id INTEGER NOT NULL,
	dense_mosaic BOOL NOT NULL,
	PRIMARY KEY (calibration_source_mosaic_id),
	UNIQUE (calibration_source_mosaic_id,calibration_source_mosaic_group_id),
	FOREIGN KEY(tem_camera_image_mosaic_id) REFERENCES tem_camera_image_mosaics (tem_camera_image_mosaic_id),
	FOREIGN KEY(calibration_source_mosaic_group_id) REFERENCES calibration_source_mosaic_groups (calibration_source_mosaic_group_id),
	CHECK (dense_mosaic IN (0,1))
);

CREATE TABLE roi_images (
	roi_image_id BIGINT NOT NULL AUTO_INCREMENT,
	image_mosaic_roi_id INTEGER NOT NULL,
	tem_camera_image_id BIGINT NOT NULL,
	current_state VARCHAR(64) NOT NULL,
	tx_hash VARCHAR(80),
	PRIMARY KEY (roi_image_id),
	FOREIGN KEY(image_mosaic_roi_id) REFERENCES image_mosaic_rois (image_mosaic_roi_id),
	FOREIGN KEY(tem_camera_image_id) REFERENCES tem_camera_images (tem_camera_image_id)
);
CREATE INDEX mosaic_roi_to_image_idx ON roi_images (image_mosaic_roi_id,tem_camera_image_id);
CREATE INDEX roi_images_tx_hash_idx ON roi_images(tx_hash);

CREATE TABLE mosaic_roi_images_clients (
       mosaic_roi_images_client_id BIGINT NOT NULL AUTO_INCREMENT,
       app_name VARCHAR(80) NOT NULL,
       roi_image_id BIGINT NOT NULL,
       current_state VARCHAR(64) NOT NULL,
       PRIMARY KEY (mosaic_roi_images_client_id),
       FOREIGN KEY(roi_image_id) REFERENCES roi_images (roi_image_id)
);
CREATE INDEX mosaic_rois_clients_app_name_idx ON mosaic_roi_images_clients(app_name);

CREATE TABLE image_stack_rois (
	image_stack_roi_id INTEGER NOT NULL AUTO_INCREMENT,
	image_stack_id INTEGER NOT NULL,
	image_mosaic_roi_id INTEGER NOT NULL,
	z_position FLOAT NOT NULL,
	rotation INTEGER,
	PRIMARY KEY (image_stack_roi_id),
	FOREIGN KEY(image_stack_id) REFERENCES image_stacks (image_stack_id),
	FOREIGN KEY(image_mosaic_roi_id) REFERENCES image_mosaic_rois (image_mosaic_roi_id)
);

CREATE TABLE acq_time_rois (
	acq_time_roi_id INTEGER NOT NULL AUTO_INCREMENT,
	tem_camera_image_mosaic_id INTEGER NOT NULL,
	image_mosaic_roi_id INTEGER NOT NULL,
	acq_region_name VARCHAR(10),
	nominal_section_number INTEGER,
	PRIMARY KEY (acq_time_roi_id),
	FOREIGN KEY(tem_camera_image_mosaic_id) REFERENCES tem_camera_image_mosaics (tem_camera_image_mosaic_id),
	FOREIGN KEY(image_mosaic_roi_id) REFERENCES image_mosaic_rois (image_mosaic_roi_id)
);
CREATE INDEX roi_section_idx ON acq_time_rois (nominal_section_number);

CREATE TABLE gomigrate (
       id INTEGER NOT NULL AUTO_INCREMENT,
       migration_id BIGINT NOT NULL,
       PRIMARY KEY (id),
       UNIQUE KEY migration_id (migration_id)
);
