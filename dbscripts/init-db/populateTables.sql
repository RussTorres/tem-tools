INSERT INTO people (username, first_name, last_name, email, notes) VALUES
   ('bockd', 'Davi', 'Bock', 'bockd@janelia.hhmi.org', NULL),
   ('perlmane', 'Eric', 'Perlman', 'perlmane@janelia.hhmi.org', NULL),
   ('fetterr', 'Rick', 'Fetter', 'fetterr@janelia.hhmi.org', NULL),
   ('robinsonc', 'Cam', 'Robinson', 'robinsonc@janelia.hhmi.org', NULL),
   ('zhengz11', 'Zheng', 'Zhihao', 'zhengz11@janelia.hhmi.org', NULL),
   ('milkied', 'Dan', 'Milkie', 'd.milkie@colemantech.com', NULL),
   ('lauritzens', 'Scott', 'Lauritzen', 'lauritzen@janelia.hhmi.org', NULL),
   ('torrensm', 'Omar', 'Torrens', 'o.torrens@colemantech.com', NULL),
   ('iyern', 'Nirmala', 'Iyer', 'iyern@janelia.hhmi.org', NULL),
   ('nichols', 'Matthew', 'Nichols', 'nicholsm@janelia.hhmi.org', NULL);


INSERT INTO tem_manufacturers (name, notes) VALUES ('FEI', NULL);


INSERT INTO tem_scintillator_manufacturers (name, notes) VALUES
   ('Grant Scientific', NULL),
   ('AMT', NULL);


INSERT INTO temca_types (temca_type_id, name, number_of_cameras, notes) VALUES
   (1,'single_camera',1,'single camera'),
   (2,'two_by_two_non_overlappinyg',4,'non-overlapping 2x2 array');


INSERT INTO tem_camera_manufacturers (tem_camera_manufacturer_id, name, notes) VALUES
   (1,'Fairchild/BAE',NULL),
   (2,'Gatan',NULL);


INSERT INTO tems (tem_id, tem_manufacturer_id, visible_id, name, notes) VALUES
   (1, 1, 'D1191', NULL, 'Array #1 (1E.344.2)'),
   (2, 1, 'D1127', NULL, 'Array #2 (1C.367)'),
   (3, 1, 'D1192', NULL, 'Autoloader Testbed (1C.369)');


INSERT INTO tem_scintillator_types (tem_scintillator_manufacturer_id, name, notes) VALUES
   (1, '10 mg/cm2 P43 on 5um mylar', NULL),
   (2, 'AMT XR60C', NULL);


INSERT INTO tem_camera_types (tem_camera_type_id, tem_camera_manufacturer_id, name, notes) VALUES
  (1, 1, 'SciMOS 2051 Model F2', NULL),
  (2, 2, 'OneView', NULL);


INSERT INTO tem_cameras (tem_camera_id, tem_camera_type_id, visible_id, notes) VALUES
  (1,1,'???','Array 1 Camera 0'),
  (2,1,'???','Array 1 Camera 1'),
  (3,1,'???','Array 1 Camera 2'),
  (4,1,'???','Array 1 Camera 3'),
  (5,1,'1232-1134','Array 2 Camera 0'),
  (6,1,'1232-1135','Array 2 Camera 1'),
  (7,1,'1232-1132','Array 2 Camera 2'),
  (8,1,'1136-1063','Array 2 Camera 3'),
  (9,1,'???','Array 3 Camera 0'),
  (10,2,'???','OneView');


INSERT INTO tem_pc_ip_addresses VALUES
   (1,1,'\ne22','2014-01-01 00:00:00',NULL),
   (2,2,'\ne2<','2014-01-01 00:00:00',NULL),
   (3,3,'\ne2F','2014-01-01 00:00:00',NULL),
   (4,1,'\0\0\0\0','2011-01-01 00:00:00',NULL);


INSERT INTO tem_scintillators (tem_scintillator_type_id, visible_id, notes) VALUES
   (1, NULL, NULL),
   (1, NULL, NULL),
   (1, NULL, NULL),
   (2, NULL, NULL),
   (2, NULL, NULL);


INSERT INTO temcas (temca_id, tem_id, tem_scintillator_id, temca_type_id, start_date, end_date, notes) VALUES
   (1,1,1,2,'2014-01-01 00:00:00','2014-06-23 00:00:00','Initial 2014 configuration of Array 1.'),
   (2,2,3,2,'2014-01-01 00:00:00',NULL,'Initial 2014 configuration of Array 2.'),
   (3,1,1,2,'2014-06-23 00:00:00','2014-07-30 00:00:00','Possible camera realignment due to Davi cable swaps.'),
   (4,3,4,1,'2014-01-01 00:00:00','2015-04-24 00:00:00','Placeholder configuration for scope #3 during autoloader testing.'),
   (5,1,2,2,'2014-07-30 00:00:00','2014-09-15 00:00:00','Scintillator replacement by Davi.'),
   (6,1,2,2,'2014-09-15 00:00:00','2014-09-26 00:00:00','Cable swap on camera1 to 6m cables.'),
   (7,1,2,2,'2014-09-26 00:00:00','2015-06-18 00:00:00','Focus change of array1-cam4.'),
   (8,3,4,1,'2015-04-24 00:00:00','2015-06-01 00:00:00','Camera adjustment by Davi.'),
   (9,3,4,1,'2015-06-01 00:00:00','2015-06-18 00:00:00','Camera adjustment by Davi.'),
   (10,3,4,1,'2015-06-18 00:00:00','2015-07-13 00:00:00','Swap with array 1.'),
   (11,1,2,1,'2015-06-18 00:00:00','2016-03-30 00:00:00','OneView test setup, old camera moved to autoloader'),
   (12,3,5,1,'2015-07-13 00:00:00','2016-03-30 00:00:00','Autoloader scintillator replacement'),
   (13,3,5,1,'2016-03-30 00:00:00',NULL,'AUtoloader OneView Installation'),
   (14,1,2,2,'2016-03-30 00:00:00',NULL,'Rebuild of Array 1');

INSERT INTO tem_camera_configurations (tem_camera_configuration_id, temca_id, tem_camera_id, tem_camera_number, tem_camera_array_col, tem_camera_array_row, tem_camera_width, tem_camera_height, notes) VALUES
   (1,1,1,0,0,0,2560,2160,NULL),
   (2,1,2,1,1,0,2560,2160,NULL),
   (3,1,3,2,0,1,2560,2160,NULL),
   (4,1,4,3,1,1,2560,2160,NULL),
   (5,3,1,0,0,0,2560,2160,NULL),
   (6,3,2,1,1,0,2560,2160,NULL),
   (7,3,3,2,0,1,2560,2160,NULL),
   (8,3,4,3,1,1,2560,2160,NULL),
   (9,5,1,0,0,0,2560,2160,NULL),
   (10,5,2,1,1,0,2560,2160,NULL),
   (11,5,3,2,0,1,2560,2160,NULL),
   (12,5,4,3,1,1,2560,2160,NULL),
   (13,6,1,0,0,0,2560,2160,NULL),
   (14,6,2,1,1,0,2560,2160,NULL),
   (15,6,3,2,0,1,2560,2160,NULL),
   (16,6,4,3,1,1,2560,2160,NULL),
   (17,7,1,0,0,0,2560,2160,NULL),
   (18,7,2,1,1,0,2560,2160,NULL),
   (19,7,3,2,0,1,2560,2160,NULL),
   (20,7,4,3,1,1,2560,2160,NULL),
   (21,11,10,0,0,0,2560,2160,NULL),
   (22,14,1,0,0,0,2560,2160,NULL),
   (23,14,2,1,1,0,2560,2160,NULL),
   (24,14,3,2,0,1,2560,2160,NULL),
   (25,14,4,3,1,1,2560,2160,NULL),
   (26,2,5,0,0,0,2560,2160,NULL),
   (27,2,6,1,1,0,2560,2160,NULL),
   (28,2,7,2,0,1,2560,2160,NULL),
   (29,2,8,3,1,1,2560,2160,NULL),
   (30,4,9,0,0,0,2560,2160,NULL),
   (31,8,9,0,0,0,2560,2160,NULL),
   (32,9,9,0,0,0,2560,2160,NULL),
   (33,10,1,0,0,0,2560,2160,NULL),
   (34,12,1,0,0,0,2560,2160,NULL),
   (35,13,10,0,0,0,2560,2160,NULL);

INSERT INTO gomigrate (migration_id) VALUES
   (1),
   (2);
