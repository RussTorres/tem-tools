# Overview.

Image catcher is a service designed to capture images sent by the microscopy application and store
their content in scality. The service also records the acquisition and tile metadata in a MySQL database.

## Installation

### Prerequisites:
* MySQL
* GO lang - to install see GO installation instructions [here](https://golang.org/doc/install)
* Building/Installing the application requires [flatbuffers](https://google.github.io/flatbuffers/). On Mac OSX
this can be installed using `brew install flatbuffers`. On linux distributions this may need to be built. On
Scientific Linux 6 building flatbuffers may require a newer version of CMake than the one that comes with the distribution.
* JFS/Scality - the current implementation relies on [Janelia File Services (JFS)](https://github.com/JaneliaSciComp/janelia-file-services.git) to store data in [Scality](http://www.scality.com/) which is the current
object store used for persisting the images.

### Steps

* Clone this repository (branch goinac)

```
git clone -b goinac git@github.com:dbock/bocklab.git image_catcher
cd image-catcher/arthurb/image_capture_service
```

* Create the database, the user and the schema

```
> mysql -u root -p
mysql> create database image_capture;
mysql> create user image_capture identified by 'image_capture';
mysql> grant all on image_capture.* to 'image_capture' identified by 'image_capture';
mysql> exit
```

* Test the database access (optional)

```
> mysql -u image_capture -p
mysql> exit
```

* Create the database schema

`
make init-db
`
if the database connection parameters differ from the default ones you can specify them in the command line using -e flag (checkout make documentation)
`
make -e IMAGECAPTURE_DB=image_capture_test -e IMAGECAPTURE_USER=image_capture -e IMAGECAPTURE_PASS=image_capture init-db
`

* Reset the database

`
make reset-db
`
or if the connection parameters differ:
`
make -e IMAGECAPTURE_DB=image_capture_test -e IMAGECAPTURE_USER=image_capture -e IMAGECAPTURE_PASS=image_capture reset-db
`

* Migrate the database

To migrate the database create a migration SQL file in the dbscripts/migrate-db and run
`
make upgrade-db
`
The migrations must be SQL files should be created in pairs (one for upgrade and one for downgrade) and should be named using the following template:

{{ id }}_{{ name }}_{{ "up" or "down" }}.sql

where id is an integer corresponding to the order in which the migration needs to be applied (starting with 1)
name can be a descriptive string
up/down suffix specify the type of migration


For example to update the temcas create 2 files (this example assumes that this is the 9th DB migration):

9_update_temcas_up.sql
```
update temcas set end_date = now() where temca_id = 14;
```
9_update_temcas_down.sql
```
update temcas set end_date = NULL where temca_id = 14;
```

then run:
`make upgrade-db`

* Build the service

```
make build-client build-service
```

* To run the unit and integration tests first you will need to create a test database named image_capture_test,
grant permissions to 'image_capture' user and create the schema:

```
> mysql -u root -p
mysql> create database image_capture_test;
mysql> grant all on image_capture.* to 'image_capture' identified by 'image_capture';
mysql> exit

make -e IMAGECAPTURE_DB=image_capture_test reset-db test
```

### Configuration

Create a local JSON configuration file based on `config.json` and overwrite the properties
based on your local configuration. Some properties that may need to be overwritten are:

* <b>JFS\_MONGO\_URL</b> - JFS database
* <b>JFS\_MONGO\_USER</b> - JFS database user
* <b>JFS\_MONGO\_PASSWORD</b> - JFS database user password
* <b>JFS\_MONGO\_USER</b> - JFS database user
* <b>JFS\_MONGO\_PASSWORD</b> - JFS database user password
* <b>DB\_HOST</b> - MySQL database host
* <b>DB\_NAME</b> - MySQL database name
* <b>DB\_USER</b> - MySQL database user
* <b>DB\_PASSWORD</b> - MySQL database password
* <b>TILE\_PROCESSING\_QUEUE\_SIZE</b> - the size of tile metadata processing queue
* <b>TILE\_PROCESSING\_WORKERS</b> - the number of workers serving the tile processing queue
* <b>CONTENT\_STORE\_QUEUE\_SIZE</b> - the size of content processing (sending to scality) queue
* <b>CONTENT\_STORE\_WORKERS</b> - the number of workers serving the content processing queue
* <b>SCALITY\_RINGS\_MAPPING</b> - defines the actual scality hosts if scality sproxy is not running on the
same host as the imagecatcher service
* <b>IMAGE\_LAB\_DRIVES\_MAP</b> - defines the mapping of the windows drives from the acquisition computer to HTTP URLs

### Setting up the calibrations

Calibrations can be imported from JSON files containing the transformation reference ID as well as the expanded transformations. If there is more
than one calibration file you need to import all calibration files separately. Below there is an example of importing three calibration files
generated at different times.

`
./imagecatchertools -tool import-calibration -calibration /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/141028_lens_correction_with_offset.json -mask_root /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/temp_render_masks
./imagecatchertools -tool import-calibration -calibration /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/150630_lens_correction_with_offset.json -mask_root /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/temp_render_masks
./imagecatchertools -tool import-calibration -calibration /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/150921_lens_correction_with_offset.json -mask_root /tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/temp_render_masks
`

### Running the service

`./imagecatcher -h` will display all available options

The service uses [google logger](https://godoc.org/github.com/golang/glog) so `-h` flag will also display all logging options
For example to run using the default configuration with some properties overwritten in config.local.json,
and the HTTP server listening on 5001 and the TCP raw server listening on 5002 you can run:

`./imagecatcher -config config.json -config config.local.json -httpserver :5001 -tcpserver :5002 -logtostderr`

During development and testing one can simply run:

`go run src/main.go -config config.json,config.json.local -cpuprofile cpu.profile`

* To run the benchmarks:

`make benchmarks`

or to run only a subset:

`go test -v  -benchmem -benchtime 10s -parallel 8 imagecatcher/service -bench "BenchmarkDbTCP.+" -run None`

## ImageCatcher API

----
#### Starting an acquisition

* _URL_:                http://host[:port]/service/v1/start-acquisition
* _METHOD_:             POST
* _Accept_:             multipart/form-data
* _HTTP Request Body_:  Multipart encoded body

       Field Multipart Name:  "acq-inilog"
       Field Multipart Value: name and content of the acquision log INI file. The acquistion log name _MUST_ start with the acquisitionUID
       which is expected to be numeric and separated from the rest of the name using an underscore '_', e.g. "123_40x168_logfile.ini"

* _Response Encoding_:  _Content-Type_:  application/json

* _Success Response_:
        _Status_:       200
        _Content_:      {"uid": 123}, where 123 is the acquisition UID which is extracted from the name of the acquisition log file name

* _Error Response_:
        _Status_:       400
        _Content_:      {"errormessage": "the error message"}

* _Example using curl_:

    `curl -i -X POST http://imagecatcher:5001/service/v1/start-acquisition -F acq-inilog=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_logfile.ini`

    ```
    {"uid":150407200041}
    ```

* _Example using the imagecatcher client_:

    `./imagecatcherclient -action send-acq-log -service-url http://imagecatcher:5001 -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1504/150407200041_40x168`

----
#### Create regions of interest

* _URL_:                http://host[:port]/service/v1/create-rois/:acquisition-id
* _METHOD_:             POST
* _Accept_:             multipart/form-data
* _HTTP Request Body_:  Multipart encoded body

       Field Multipart Name:  "roi-tiles"
       Field Multipart Value: name and content of the ROI tiles CSV file
       Field Multipart Name:  "roi-spec"
       Field Multipart Value: name and content of the ROI spec INI file

* _Response Encoding_:  _Content-Type_:  application/json

* _Success Response_:
        _Status_:       200
        _Content_:      {}

* _Error Response_:
        _Status_:       400
        _Content_:      {"errormessage": "the error message"}

* _Example using curl_:

    `curl -i -X POST http://imagecatcher:5001/service/v1/create-rois/150407200041 -F roi-tiles=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_tiles.csv -F roi-spec=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_spec.ini`

* _Example using the imagecatcher client_:

    `./imagecatcherclient -action create-rois -service-url http://imagecatcher:5001 -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1504/150407200041_40x168`

----
#### Capturing/Persisting a tile

* _URL_:                http://host[:port]/service/v1/capture-image-content/:acquisition-id?tile-filename="tile file name that encodes camera,col,row and frame info"
* _METHOD_:             PUT | POST
* _Accept_:             application/octet-stream
* _HTTP Request Parameters_:

       Path Parameter: :acquisition-id is the acquisition UID returned by start-acquisition request
       Query Parameter: tile-filename="tile file name that encodes camera,col,row and frame info"

       For stable images tile-filename must comform with regexp: "col(\\d+)_row(\\d+)_cam(\\d+)\\.tif"
       Example: tile-filename=col0006_row0017_cam1.tif
       
       For drift images tile-filename must conform with regex: "Dump_cam(\\d)_frame(\\d+)_col(\\d+)_row(\\d+)\\.tif"
       Example: tile-filename=Dump_cam0_frame02948_col0012_row0156.tif

* _HTTP Request Body_:  content of the tile image

Notes: It's also possible to send the content using multipart/form-data and encode the tile file name and content as a multipart field (See curl example).

* _Response Encoding_:  _Content-Type_:  application/json

* _Success Response_:
        _Status_:       200
        _Content_:      `{"tile_camera": 1, "tile_col": 6, "tile_frame": -1, "tile_id": 834, "tile_jfs_key":"", "tile_jfs_path":"", "tile_row":17}`

* _Error Response_:
        _Status_:       400
        _Content_:      {"errormessage": "the error message"}

        _Status_:       500
        _Content_:      {"errormessage": "processing error message"}

* _Example using curl_:

     `curl -X PUT http://imagecatcher:5001/service/v1/capture-image-content/150407200041?tile-filename=col0006_row0017_cam1.tif --data-binary @/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/col0006/col0006_row0017_cam1.tif`

     `curl -X POST http://imagecatcher:5001/service/v1/capture-image-content/150407200041?tile-filename=col0006_row0017_cam1.tif --data-binary @/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/col0006/col0006_row0017_cam1.tif`

     `curl -X POST http://imagecatcher:5001/service/v1/capture-image-content/150407200041 -F tile-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/col0006/col0006_row0017_cam1.tif`

     ```
     {"tile_camera":1,"tile_col":6,"tile_file_checksum":"","tile_frame":-1,"tile_id":834,"tile_jfs_key":"","tile_jfs_path":"","tile_row":17}
     ```
     
     `curl -X PUT http://imagecatcher:5001/service/v1/capture-image-content/150407200041?tile-filename=Dump_cam0_frame02948_col0012_row0156.tif --data @/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/Dump__150407200041_40x168/Dump_cam0_frame02948_col0012_row0156.tif`

     ```
     {"tile_camera":0,"tile_col":12,"tile_file_checksum":"","tile_frame":2948,"tile_id":1,"tile_jfs_key":"","tile_jfs_path":"","tile_row":156}
     ```

* _Example using the imagecatcher client_:

     `./imagecatcherclient -action send-tile -send-method HTTP-PUT -service-url http://imagecatcher:5001 -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1504/150407200041_40x168 -camera 0 -row 17 -col 4`

----
#### Ending an acquisition

* _URL_:                http://host[:port]/service/v1/end-acquisition/:acquisition-id
* _METHOD_:             POST
* _Accept_:             multipart/form-data
* _HTTP Request Parameters_:

       Path Parameter: :acquisition-id is the acquisition UID returned by start-acquisition request

* _HTTP Request Body_:  Multipart encoded body containing ancillary files associated with the acquisition. This is optional if we want to send
other files associated with the acquisition that are closed only at the end.

       Field Multipart Name:  "ancillary-file"
       Field Multipart Value: name and content of the ancillary file(s), e.g.: "150407200041_40x168_ROI_spec.ini", "150407200041_40x168_ROI_tiles.csv", "150407200041_40x168_mosaic.png"

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:
        
        ```
        {"af_results":[
           {
               "id":<ancillary file id>,
               "jfs_key":"<scality key>",
               "jfs_path":"<JFS access path>",
               "name":"<ancillary file name as it comes in the multipart form request>",
               "path":"<persisted ancillary file name>"
           },
           {
               "errormessage": "<processing error message>",
               "name":"<ancillary file name as it comes in the multipart form request>"
           }]
        }
        ```

* _Success Response_:
        _Status_:       200
        _Content_:      JSON body containing the list of persisted ancillary files

* _Error Response_:
        _Status_:       400
        _Content_:      JSON body containing a list of both succesfully imported files and of the ones that encountered errors

* _Example using curl_:

    `curl -i -X POST http://imagecatcher:5001/service/v1/end-acquisition/150407200041 -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_spec.ini -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_tiles.csv -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_mosaic.png`

    ```
    {"af_results":[{"id":3424,"jfs_key":"a940a376-16ca-11e6-84dc-acbc328a44c1","jfs_path":"/FAFB/acquisition/150407200041/150407200041_40x168_ROI_spec.ini","name":"150407200041_40x168_ROI_spec.ini","path":"150407200041_40x168_ROI_spec.ini"},{"id":3425,"jfs_key":"95f8f93c-d053-47d3-94e7-22d38e60e3cb","jfs_path":"/FAFB/acquisition/150407200041/150407200041_40x168_ROI_tiles.csv","name":"150407200041_40x168_ROI_tiles.csv","path":"150407200041_40x168_ROI_tiles.csv"},{"id":3426,"jfs_key":"f541b9b4-292d-483a-aa8f-fc3e1e74ff4e","jfs_path":"/FAFB/acquisition/150407200041/150407200041_40x168_mosaic.png","name":"150407200041_40x168_mosaic.png","path":"150407200041_40x168_mosaic.png"}]}
    ```
----
#### Store one or more ancillary files associated with the acquisition

The parameters and the handling of this request is identical with the handling of the end-acquisition request - the only thing that differs is the base URI.

* _URL_:                http://host[:port]/service/v1/store-ancillary-files/:acquisition-id
* _METHOD_:             POST
* _Accept_:             multipart/form-data
* _HTTP Request Parameters_:

       Path Parameter: :acquisition-id is the acquisition UID returned by start-acquisition request

* _HTTP Request Body_:  Multipart encoded body

       Field Multipart Name:  "ancillary-file"
       Field Multipart Value: name and content of the ancillary file(s), e.g.: "150407200041_40x168_ROI_spec.ini", "150407200041_40x168_ROI_tiles.csv", "150407200041_40x168_mosaic.png"

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:
        
        `
        {"af_results":[
           {
               "id":<ancillary file id>,
               "jfs_key":"<scality key>",
               "jfs_path":"<JFS access path>",
               "name":"<ancillary file name as it comes in the multipart form request>",
               "path":"<persisted ancillary file name>"
           },
           {
               "errormessage": "<processing error message>",
               "name":"<ancillary file name as it comes in the multipart form request>"
           }]
        }
        `

* _Success Response_:
        _Status_:       200
        _Content_:      JSON body containing the list of persisted ancillary files

* _Error Response_:
        _Status_:       400
        _Content_:      JSON body containing a list of both succesfully imported files and of the ones that encountered errors

* _Example using curl_:

    `curl -i -X POST http://imagecatcher:5001/service/v1/store-ancillary-files/150407200041 -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_spec.ini -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_ROI_tiles.csv -F ancillary-file=@/tier2/flyTEM/data/FAFB00/1504/150407200041_40x168/150407200041_40x168_mosaic.png`

----

#### Feeding an entire acquistion from the file system

If the entire acquisition exists on the file system one can use `feed-acquisition.sh` to send the entire acquisition to the imagecatcher service:
`
 sh feed-acquisition.sh -as imagecatcher:5001 -ad /tier2/flyTEM/data/FAFB00/1504/150407200041_40x168
`
The script assumes that the directory contains an acquisition INI log ending in `_logfile.ini`, the regions of interests are in `_ROI_spec.ini` and `_ROI_tiles.csv` files and tiles are in _colXXXX_ subdirectories.


Another way to send an acquisition is by using the imageacquisitionclient, which in addition, allows the user to specify a send rate and
a specific camera from which to send the data. The imageacquisitionclient uses the DAQ Timing CSV file to send the data and for sending data from
all cameras the camera parameter should be -1.

Examples:

* send acquisition data for all cameras from '/tier2/flyTEM/data/FAFB00/1511/151118151843_100x176' directory using HTTP PUT every 150ms

`
./imagecatcherclient -acq-dir-url /tier2/flyTEM/data/FAFB00/1511/151118151843_100x176 -action send-acq -camera -1 -send-method HTTP-PUT -rate 150
`

* send acquisition data only from camera 2 from '/tier2/flyTEM/data/FAFB00/1511/151118151843_100x176' directory using TCP every 250ms

`
./imagecatcherclient -acq-dir-url /tier2/flyTEM/data/FAFB00/1511/151118151843_100x176 -action send-acq -camera 2 -send-method TCP-SEND -rate 250
`

----
#### Querying and Retrieving the Data

----
##### Query the acquisitions

* _URL_:                http://host[:port]/service/v1/acquisitions
* _METHOD_:             GET
* _HTTP Request Parameters_:
        Optional Query Parameters:
                 'acqid' acquisition UID
                 'sample' sample name (as defined in the acquisition INI file)
                 'project' project name (as defined in the acquisition INI file)
                 'owner' data owner (as defined in the acquisition INI file)
                 'stack' stack name (as defined in the acquisition INI file)
                 'acq-from', 'acq-to' optional time interval parameters that specify the interval for the mosaic aquired time. The mosaic acquired timestamp
                 is typically specified in the acquisition INI file.
                 The timestamp format is: 'yyyyMMdd`
                 'exists-tile-in-state', 'all-tiles-in-state' parameters that allow searching acquisitions that have at least or all tiles, respectively,
                 in the specified state
                 'offset', 'length' pagination parameters that specify the start record index and the maximum number of records to be retrieved 

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:

        `
        {
          "acquisitions": [{
            "IniFileID": 1,
            "ImageMosaicID": 1,
            "Acquired": "2015-12-15T05:15:54Z",
            "Inserted": "2016-07-07T14:22:49Z",
            "Completed": "2016-07-07T14:23:17Z",
            "AcqUID": 151215051552,
            "NumberOfCameras": 1,
            "XSmallStepPix": 1776,
            "YSmallStepPix": 1976,
            "XBigStepPix": 1776,
            "YBigStepPix": 1976,
            "NXSteps": 14,
            "NYSteps": 16,
            "NTargetCols": 0,
            "NTargetRows": 0,
            "XCenter": -0.00018950167577713728,
            "YCenter": 0.001359121291898191,
            "YTem": 0,
            "PixPerUm": 237.89999389648438,
            "Magnification": 4800,
            "TemIPAddress": "",
            "URL": "",
            "SampleName": "FAFB",
            "ProjectName": "",
            "ProjectOwner": "",
            "StackName": "",
            "MosaicType": "",
            "RoiTilesFile": "",
            "RoiSpecFile": "",
            "MicroscopistName": "Autoloader",
            "Notes": "",
            "IniContent": ""
          }],
          "length": 1
        }
        `

----
##### Query the tiles of a specific acquisition

* _URL_:                http://host[:port]/service/v1/acquisition/:acquisition-id/tiles
* _METHOD_:             GET
* _HTTP Request Parameters_:
       Path Parameter: :acquisition-id is the acquisition UID returned by start-acquisition request
       Query Parameters
                 'col' tile column
                 'row' tile row
                 'camera' camera number used for tile image acquisition
                 'tile-from', 'tile-to' specifies the time interval for tile persisted timestamp.
                 The timestamp format is: 'yyyyMMddHHmmss[.SSS[SSS]]`
                 'offset', 'length' pagination parameters that specify the start record index and the maximum number of records to be retrieved

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:

        `
        {
          "length": 93,
          "tiles": [
            {
              "tile_acq_id": 151215051552,
              "tile_camera": 0,
              "tile_col": 2,
              "tile_file_checksum": "6632285fc686a914e3b80e3994316c9b00000000",
              "tile_frame": -1,
              "tile_id": 1,
              "tile_jfs_key": "6a5aafc4-3884-11e6-9772-acbc328a44c1",
              "tile_jfs_path": "/acquisitions/151215051552/col0002_row0002_cam0.tif",
              "tile_row": 2,
              "tile_temca": 0
            },
            ...
          ]
        }
        `

----
##### Retrieve tile metadata for a tile identified by acquisition id, column and row

* _URL_:                http://host[:port]/service/v1/acquisition/:acquisition-id/tile/:col/:row
* _METHOD_:             GET
* _HTTP Request Parameters_:
       Path Parameters:
            :acquisition-id is the acquisition UID returned by start-acquisition request
            :col tile column
            :row tile row

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:

        `
        {
              "tile_acq_id": 151215051552,
              "tile_camera": 0,
              "tile_col": 2,
              "tile_file_checksum": "6632285fc686a914e3b80e3994316c9b00000000",
              "tile_frame": -1,
              "tile_id": 1,
              "tile_jfs_key": "6a5aafc4-3884-11e6-9772-acbc328a44c1",
              "image_url": "http://sc1-jrc:81/proxy/bparc/6a5aafc4-3884-11e6-9772-acbc328a44c1",
              "tile_jfs_path": "/acquisitions/151215051552/col0002_row0002_cam0.tif",
              "tile_row": 2,
              "tile_temca": 0,
	      "tile_acquired": "20150304092522.000000"
	}
        `

----
##### Retrieve tile image for a tile identified by acquisition id, column and row

* _URL_:                http://host[:port]/service/v1/acquisition/:acquisition-id/tile/:col/:row/content
* _METHOD_:             GET
* _HTTP Request Parameters_:
       Path Parameters:
            :acquisition-id is the acquisition UID returned by start-acquisition request
            :col tile column
            :row tile row

* _Response Encoding_:  _Content-Type_:  application/octet-stream

----
##### Query the tiles

* _URL_:                http://host[:port]/service/v1/tiles
* _METHOD_:             GET
* _HTTP Request Parameters_:
        Optional Query Parameters:
                 'acqid' acquisition UID
                 'col' tile column
                 'row' tile row
                 'camera' camera number used for tile image acquisition
         'tile-from', 'tile-to' specifies the time interval for tile persisted timestamp.
                 The timestamp format is: 'yyyyMMddHHmmss[.SSS[SSS]]`
                 'offset', 'length' pagination parameters that specify the start record index and the maximum number of records to be retrieved

* _Response Encoding_:  _Content-Type_:  application/json
        _Format_:

        `
        {
          "length": 93,
          "tiles": [
            {
              "tile_acq_id": 151215051552,
              "tile_camera": 0,
              "tile_col": 2,
              "tile_file_checksum": "6632285fc686a914e3b80e3994316c9b00000000",
              "tile_frame": -1,
              "tile_id": 1,
              "tile_jfs_key": "6a5aafc4-3884-11e6-9772-acbc328a44c1",
              "image_url": "http://sc1-jrc:81/proxy/bparc/6a5aafc4-3884-11e6-9772-acbc328a44c1",
              "tile_jfs_path": "/acquisitions/151215051552/col0002_row0002_cam0.tif",
              "tile_row": 2,
              "tile_temca": 0,
	      "tile_acquired": "20150304092522.000000"
            },
            ...
          ]
        }
        `

----
##### Update the calibrations

* _URL_:                http://host[:port]/service/v1/calibrations
* _METHOD_:             POST
* _HTTP Request Body_:
   content of the JSON calibration file
* _Response Encoding_:  _Content-Type_:  application/json
   list of newly created calibrations

Example:

`
curl -X POST http://imagecatcher:5001/service/v1/calibrations --data @/tier2/flyTEM/eric/working_sets/160208_FAFB_V12_delta/141028_lens_correction_with_offset.json
`

----
##### Get all calibrations

* _URL_:                http://host[:port]/service/v1/calibrations?[offset=offset_value][&length=length_value]
* _METHOD_:             GET
* _HTTP Request Parameters_:

  	Query Parameter: offset=index of the first retrieved record
  	Query Parameter: length=maximum number of records retrieved

* _Response Encoding_:  _Content-Type_:  application/json
   list of all calibrations

Example:

`
curl http://imagecatcher:5001/service/v1/calibrations
`

----
##### Get calibration by name

* _URL_:                http://host[:port]/service/v1/calibration/name/:calibration_name
* _METHOD_:             GET
* _HTTP Request Parameters_:

  	Path Parameter: calibration_name name of the calibration to retrieve

* _Response Encoding_:  _Content-Type_:  application/json

  	Response Status: 200 if calibration is found
	Response Body JSON encoded calibration 

  	Response Status: 404 if no calibration with the given name exists

Example:

`
curl http://imagecatcher:5001/service/v1/calibration/name/150921offset_temca2_camera0
`
