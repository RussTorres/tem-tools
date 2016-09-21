package service

import (
	"encoding/hex"
	"fmt"
	"github.com/JaneliaSciComp/janelia-file-services/JFSGolang/jfs"
	"github.com/hashicorp/golang-lru"
	"strings"
	"time"

	"imagecatcher/config"
	"imagecatcher/dao"
	"imagecatcher/logger"
	"imagecatcher/models"
	"imagecatcher/utils"
)

const (
	pulseFileType             = "Add pulse.csv"
	controllerElapsedFileType = "Controller elapsed.csv"
	controllerTimingFileType  = "Controller timing.csv"
	daqTimingFileType         = "DAQ timing.csv"
	roiSpecFileType           = "ROI_spec.ini"
	roiTilesFileType          = "ROI_tiles.csv"
	logfileFileType           = "logfile.ini"
	mosaicFileType            = "mosaic.png"
	statisticsFileType        = "statistics.csv"
	storedImageListFileType   = "stored_image_list.csv"
	jfsPingFileType           = "jfs ping"
	unknownFileType           = "unknown"
)

type imageCatcherServiceImpl struct {
	config          config.Config
	jfsFactory      jfs.Factory
	dbHandler       dao.DbHandler
	jfsServiceCache *lru.Cache
	jfsMapping      map[string]string
}

// NewService creates an instance of an image catcher service
func NewService(dbHandler dao.DbHandler, config config.Config) (ImageCatcherService, error) {
	imageCatcherService := &imageCatcherServiceImpl{}
	err := imageCatcherService.Initialize(dbHandler, config)
	return imageCatcherService, err
}

func (s *imageCatcherServiceImpl) Initialize(dbHandler dao.DbHandler, config config.Config) error {
	s.config = config

	// Initialize JFS Factory
	jfsUser := config.GetStringProperty("JFS_MONGO_USER", "")
	jfsPassword := config.GetStringProperty("JFS_MONGO_PASSWORD", "")
	scalityRingsMapping := config.GetStringMapProperty("SCALITY_RINGS_MAPPING")
	jfsDbUrls := config.GetStringArrayProperty("JFS_MONGO_URL")
	logger.Debugf("Connect to JFS database(s) %v", jfsDbUrls)
	jfsFactory, err := jfs.GetFactoryInstance(
		jfsUser,
		jfsPassword,
		jfsDbUrls,
		scalityRingsMapping)
	if err != nil {
		return fmt.Errorf("Error initializing JFS Factory: %v", err)
	}
	if err = s.initializeJFS(jfsFactory); err != nil {
		return err
	}
	s.dbHandler = dbHandler

	return err
}

func (s *imageCatcherServiceImpl) initializeJFS(f jfs.Factory) error {
	var err error
	s.jfsFactory = f
	s.jfsMapping = s.config.GetStringMapProperty("JFS_MAPPING")
	if s.jfsServiceCache, err = lru.New(s.config.GetIntProperty("JFS_SERVICE_CACHE_LEN", 20)); err != nil {
		return fmt.Errorf("Error initializing JFS service cache")
	}
	return nil
}

func (s *imageCatcherServiceImpl) getAcqFileService(acq *models.Acquisition) (jfs.FileService, error) {
	jfsCollectionName := s.jfsMapping[acq.SampleName]
	if jfsCollectionName == "" {
		jfsCollectionName = s.jfsMapping[acq.ProjectName]
	}
	if jfsCollectionName == "" {
		jfsCollectionName = s.jfsMapping[acq.ProjectOwner]
	}
	if jfsCollectionName == "" {
		jfsCollectionName = acq.ProjectOwner
	}
	return s.getFileService(jfsCollectionName)
}

func (s *imageCatcherServiceImpl) getFileService(jfsCollectionName string) (fileService jfs.FileService, err error) {
	cachedFileService, found := s.jfsServiceCache.Get(jfsCollectionName)
	if found {
		return cachedFileService.(jfs.FileService), nil
	}
	logger.Debugf("Open fileservice %s", jfsCollectionName)
	fileService = s.jfsFactory.GetFileService(jfsCollectionName)
	if fileService == nil {
		return nil, fmt.Errorf("No JFS service found for %s ", jfsCollectionName)
	}
	if err = fileService.Open(); err != nil {
		return fileService, fmt.Errorf("Error opening JFS service for %s: %v", jfsCollectionName, err)
	}
	s.jfsServiceCache.Add(jfsCollectionName, fileService)
	return fileService, nil
}

// CreateAcquisition creates an acquisition from an acquisition log file.
func (s *imageCatcherServiceImpl) CreateAcquisition(acqLog *FileParams) (*models.Acquisition, error) {
	acq, err := utils.ParseAcqIniLog(acqLog.Content)
	if err != nil {
		return acq, err
	}
	acq.AcqUID, err = utils.GetAcqUIDFromURL(acqLog.Name)
	if err != nil {
		return acq, err
	}

	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return acq, err
	}

	if err = dao.CreateAcqLog(acq, session); err == nil {
		var imageMosaic *models.TemImageMosaic
		imageMosaic, err = dao.CreateImageMosaic(acq, session)
		acq.ImageMosaicID = imageMosaic.ImageMosaicID
	}
	session.Close(err)
	return acq, err
}

// CreateROIs creates mosaic ROIs from the corresponding ROI INI and CSV files.
func (s *imageCatcherServiceImpl) CreateROIs(acqMosaic *models.TemImageMosaic, roiFiles *ROIFiles) error {
	// create ancillary file for ROI Spec
	s.persistAncillaryFile(acqMosaic, roiSpecFileType, &roiFiles.RoiSpec)

	// create ancillary file for ROI Tiles
	s.persistAncillaryFile(acqMosaic, roiTilesFileType, &roiFiles.RoiTiles)

	// process ROI Spec
	regionToAcqROIMap, err := utils.ParseTileSpec(roiFiles.RoiSpec.Content)
	if err != nil {
		return err
	}
	if err := s.createROISections(acqMosaic, regionToAcqROIMap); err != nil {
		return err
	}

	// process ROI Tiles
	roiTiles, err := utils.ParseROITiles(roiFiles.RoiTiles.Content)
	if err == nil {
		err = s.createROITiles(acqMosaic, roiTiles)
	}

	return err
}

// CreateAncillaryFile creates an ancillary file for the specified mosaic. If the ancillary file is a ROI INI spec or a ROI CSV file
// it also creates the corresponding regions of interest.
func (s *imageCatcherServiceImpl) CreateAncillaryFile(acqMosaic *models.TemImageMosaic, f *FileParams) (*models.FileObject, error) {
	fileType := extractAncillaryFileType(f.Name)
	fObj, err := s.persistAncillaryFile(acqMosaic, fileType, f)
	if err != nil {
		return fObj, err
	}
	switch fileType {
	case roiSpecFileType:
		// process ROI Spec
		regionToAcqROIMap, err := utils.ParseTileSpec(f.Content)
		if err == nil {
			err = s.createROISections(acqMosaic, regionToAcqROIMap)
		}
		return fObj, err
	case roiTilesFileType:
		// process ROI Tiles
		roiTiles, err := utils.ParseROITiles(f.Content)
		if err == nil {
			err = s.createROITiles(acqMosaic, roiTiles)
		}
		return fObj, err
	default:
		return fObj, err
	}
}

func extractAncillaryFileType(fname string) string {
	r := strings.NewReplacer(
		"_cam0.csv", ".csv",
		"_cam1.csv", ".csv",
		"_cam2.csv", ".csv",
		"_cam3.csv", ".csv",
	)
	name := r.Replace(fname)
	knownFileTypes := []string{
		pulseFileType,
		controllerElapsedFileType,
		controllerTimingFileType,
		daqTimingFileType,
		roiSpecFileType,
		roiTilesFileType,
		logfileFileType,
		mosaicFileType,
		statisticsFileType,
		storedImageListFileType,
	}
	for _, ft := range knownFileTypes {
		if strings.HasSuffix(name, ft) {
			return ft
		}
	}
	return unknownFileType
}

func (s *imageCatcherServiceImpl) persistAncillaryFile(acqMosaic *models.TemImageMosaic, fileType string, f *FileParams) (*models.FileObject, error) {
	fileObject, err := s.createAncillaryFileObject(acqMosaic, f, fileType)
	if err != nil {
		return fileObject, err
	}
	jfsObj, err := s.storeContent(acqMosaic, f)
	if err != nil {
		return fileObject, err
	}
	if err = fileObject.UpdateJFSAndPathParams(f.Name, jfsObj); err != nil {
		return fileObject, err
	}
	if err = s.updateFileObj(fileObject); err != nil {
		return fileObject, fmt.Errorf("Error updating JFS parameters for %v", fileObject.Path)
	}
	return fileObject, nil
}

func (s *imageCatcherServiceImpl) createAncillaryFileObject(acqMosaic *models.TemImageMosaic, f *FileParams, fileType string) (*models.FileObject, error) {
	var err error
	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return nil, err
	}
	fileTypeID, err := dao.CreateAncillaryFileType(fileType, session)
	if err != nil {
		session.Close(err)
		return nil, err
	}
	fileObject, err := dao.CreateAncillaryFile(acqMosaic, f.Name, fileTypeID, session)
	session.Close(err)
	return fileObject, err
}

func (s *imageCatcherServiceImpl) createROISections(imageMosaic *models.TemImageMosaic, regionToAcqROIMap map[string]*models.AcqROI) error {
	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return err
	}
	for _, acqRoi := range regionToAcqROIMap {
		if _, err = dao.CreateROI(imageMosaic, acqRoi, session); err != nil {
			break
		}
	}
	session.Close(err)
	return err
}

func (s *imageCatcherServiceImpl) createROITiles(imageMosaic *models.TemImageMosaic, roiTiles []models.TemImageROI) error {
	sectionTilesMap := make(map[string][]models.TemImageROI)
	for _, rt := range roiTiles {
		sectionTiles, ok := sectionTilesMap[rt.Roi.SectionName]
		if !ok {
			sectionTiles = make([]models.TemImageROI, 0)
		}
		sectionTilesMap[rt.Roi.SectionName] = append(sectionTiles, rt)
	}

	roiSectionMap, err := s.retrieveROIsBySection(imageMosaic, sectionTilesMap)

	for section, sectionTiles := range sectionTilesMap {
		roiSection, ok := roiSectionMap[section]
		if !ok {
			logger.Errorf("WARNING: No ROI found for section %s", section)
			continue
		}
		var session dao.DbSession
		var temImage *models.TemImage
		if session, err = s.dbHandler.OpenSession(false); err != nil {
			// break the loop if the db session cannot be created
			break
		}
		for _, ti := range sectionTiles {
			// Create TemImage
			temImage, err = dao.CreateTemImage(imageMosaic, ti.Col, ti.Row, -1, session)
			if err != nil {
				break
			}
			// Create ROI Tile
			if _, err = dao.CreateTileROI(imageMosaic, temImage.ImageID, roiSection, "", session); err != nil {
				break
			}
		}
		session.Close(err)
		if err != nil {
			break
		}
	}
	return err
}

func (s *imageCatcherServiceImpl) retrieveROIsBySection(imageMosaic *models.TemImageMosaic, tilesBySection map[string][]models.TemImageROI) (map[string]*models.AcqROI, error) {
	sections := make([]string, 0, len(tilesBySection))
	for section := range tilesBySection {
		sections = append(sections, section)
	}

	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return nil, err
	}
	roiSectionMap, err := dao.RetrieveROIsBySections(imageMosaic, sections, session)
	session.Close(err)
	return roiSectionMap, err
}

// EndAcquisition - only updates the acquistion completed date.
func (s *imageCatcherServiceImpl) EndAcquisition(acqID uint64) error {
	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return err
	}

	err = dao.EndAcquisition(acqID, session)
	session.Close(err)

	return err
}

// StoreTile - persists a tile metadata for the given mosaic.
func (s *imageCatcherServiceImpl) StoreTile(acqMosaic *models.TemImageMosaic, tileParams *TileParams) (*models.TemImage, error) {
	tileImage, err := s.persistTileInfo(acqMosaic, tileParams)
	if err != nil {
		logger.Errorf("Error while persisting tile %v for %v: %v", tileParams, acqMosaic, err)
		return nil, err
	}

	return tileImage, nil
}

func (s *imageCatcherServiceImpl) persistTileInfo(acqMosaic *models.TemImageMosaic, tileParams *TileParams) (*models.TemImage, error) {
	cameraConfiguration, err := s.retrieveCameraConfigurationByTemcaIDAndCamera(acqMosaic.Temca.TemcaID, tileParams.Camera)
	if err != nil {
		logger.Errorf("Ignoring camera configuration error: %v", err)
	}

	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return nil, err
	}

	temImage, err := dao.CreateTemImage(acqMosaic, tileParams.Col, tileParams.Row, tileParams.Frame, session)

	defer func() {
		session.Close(err)
	}()

	if err != nil {
		return temImage, err
	}
	// we need to check if camera configurations are valid since we don't check for errors when we retrieve the camera configurations
	if cameraConfiguration != nil && temImage.Configuration == nil {
		temImage.Configuration = cameraConfiguration
		err = dao.UpdateTemImageCamera(temImage, session)
	} else {
		temImage.Configuration = cameraConfiguration
	}
	return temImage, err
}

// UpdateTile - updates a tile's metadata
func (s *imageCatcherServiceImpl) UpdateTile(tileImage *models.TemImage) error {
	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return err
	}

	defer func() {
		session.Close(err)
	}()
	if err = dao.UpdateTemImage(tileImage, session); err != nil {
		return err
	}
	roiUpdates, err := dao.UpdateROIState(tileImage.ImageID, models.TileReadyState, session)
	if err != nil {
		return err
	}
	if roiUpdates > 0 {
		// the tile's ROI was updated therefore it must have been found so return OK
		return nil
	}
	// no ROI update was performed so it's very likely the ROI does not exist
	// possible because this is a drift frame
	tileROI, err := dao.RetrieveTemImage(tileImage.ImageID, session)
	if err != nil {
		return err
	}
	if tileROI.Roi.AcqRoiID > 0 {
		// the tile is associated with a ROI so it's possible the state didn't need to be updated
		// because it already had the needed value so simply return OK
		logger.Infof("No new ROI will be created for %v because ROI %d is already associated with the tile", tileImage, tileROI.Roi.AcqRoiID)
		return nil
	}
	tileAcqROIs, err := dao.RetrieveTileROIsByMosaicColAndRow(tileImage.ImageMosaic.ImageMosaicID, tileImage.Col, tileImage.Row, session)
	for _, tileAcqROI := range tileAcqROIs {
		if _, roiErr := dao.CreateTileROI(&tileROI.ImageMosaic, tileROI.ImageID, tileAcqROI, models.TileReadyState, session); roiErr != nil {
			logger.Errorf("Error while creating ROI %v for %v", tileAcqROI, tileROI)
			err = roiErr
		}
	}
	return nil
}

// GetMosaic retrieves the acquisition's mosaic entity. If no mosaic is found the error result is set.
func (s *imageCatcherServiceImpl) GetMosaic(acqID uint64) (*models.TemImageMosaic, error) {
	session, _ := s.dbHandler.OpenSession(true)
	imageMosaic, err := dao.RetrieveImageMosaicByAcqUID(acqID, session)
	session.Close(err)
	if err != nil {
		return nil, err
	}
	if imageMosaic == nil {
		return nil, fmt.Errorf("No mosaic image found for acquisition id %d", acqID)
	}
	return imageMosaic, nil
}

func (s *imageCatcherServiceImpl) retrieveCameraConfigurationByTemcaIDAndCamera(temcaID int64, camera int) (*models.TemCameraConfiguration, error) {
	session, _ := s.dbHandler.OpenSession(true)
	cameraConfigurations, err := dao.RetrieveCameraConfigurationsByTemcaID(temcaID, session)
	session.Close(err)
	if err != nil {
		return nil, fmt.Errorf("No camera configuration found for temca %d: %v", temcaID, err)
	}
	cameraConfiguration := cameraConfigurations[camera]
	return &cameraConfiguration, nil
}

// StoreAcquisitionFile
func (s *imageCatcherServiceImpl) StoreAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) error {
	return s.updateFileContent(acqMosaic, f, fObj)
}

func (s *imageCatcherServiceImpl) updateFileContent(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) error {
	jfsObj, err := s.storeContent(acqMosaic, f)
	if err != nil {
		logger.Errorf("Error storing the content of %s for %v: %v", f.Name, acqMosaic, err)
		return err
	}
	if fObj == nil {
		return fmt.Errorf("Null file object passed in for updateContent")
	}
	if err = fObj.UpdateJFSAndPathParams(f.Name, jfsObj); err != nil {
		return err
	}
	return s.updateFileObj(fObj)
}

func (s *imageCatcherServiceImpl) updateFileObj(f *models.FileObject) error {
	session, err := s.dbHandler.OpenSession(false)
	if err != nil {
		return err
	}
	err = dao.UpdateFileObj(f, session)
	session.Close(err)
	return err
}

func (s *imageCatcherServiceImpl) storeContent(acqMosaic *models.TemImageMosaic, f *FileParams) (res map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Unexpcted error while storing file %v for %v", f, acqMosaic)
		}
	}()
	fileService, err := s.getAcqFileService(&acqMosaic.Acquisition)
	if err != nil {
		return res, err
	}
	filename := f.Name
	body := f.Content
	data := s.createAcqStorageContext(acqMosaic, f)
	jfsFilePath := s.getJfsFilePath(acqMosaic.AcqUID, filename)

	if res, err = fileService.Put(jfsFilePath, body, data); err != nil {
		return res, err
	}
	res["jfsPath"] = jfsFilePath
	var locationURL string
	if res["locationUrl"] != nil {
		locationURL = strings.Replace(res["locationUrl"].(string),
			"localhost",
			s.config.GetStringProperty("SCALITY_HOST", "localhost"),
			1)
		res["locationUrl"] = locationURL
	}

	return res, err
}

func (s *imageCatcherServiceImpl) createAcqStorageContext(acq *models.TemImageMosaic, f *FileParams) map[string]string {
	storageContext := map[string]string{}
	storageContext["sample"] = acq.SampleName
	if project := acq.ProjectName; project != "" {
		storageContext["project"] = project
	}
	if owner := acq.ProjectOwner; owner != "" {
		storageContext["owner"] = owner
	}
	if tileStack := acq.StackName; tileStack != "" {
		storageContext["stack"] = tileStack
	}
	if len(f.Checksum) > 0 {
		storageContext["checksum"] = hex.EncodeToString(f.Checksum)
	}

	return storageContext
}

func (s *imageCatcherServiceImpl) getJfsFilePath(acqID uint64, filename string) string {
	return fmt.Sprintf("/acquisitions/%d/%s", acqID, filename)
}

// VerifyAcquisitionFile checks if the persisted file has the same checksum as the given parameter.
func (s *imageCatcherServiceImpl) VerifyAcquisitionFile(acqMosaic *models.TemImageMosaic, f *FileParams, fObj *models.FileObject) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Unexpcted error while verifying file %v for %v", f, acqMosaic)
		}
	}()
	fileService, err := s.getAcqFileService(&acqMosaic.Acquisition)
	if err != nil {
		return err
	}
	data := s.createAcqStorageContext(acqMosaic, f)
	data["scalityKey"] = fObj.JfsKey
	logger.Debugf("Verify content: %d:%s (%d) - %v", acqMosaic.AcqUID, f.Name, f.ContentLen, data)
	return fileService.Verify(s.getJfsFilePath(acqMosaic.AcqUID, f.Name), data)
}

func (s *imageCatcherServiceImpl) GetTile(tileID int64) (*models.TemImageROI, error) {
	session, _ := s.dbHandler.OpenSession(true)
	temImage, err := dao.RetrieveTemImage(tileID, session)
	session.Close(err)

	return temImage, err
}

func (s *imageCatcherServiceImpl) GetAcquisitions(acqFilter *models.AcquisitionFilter) ([]*models.Acquisition, error) {
	session, _ := s.dbHandler.OpenSession(true)
	acquisitions, err := dao.RetrieveAcquisitions(acqFilter, session)
	session.Close(err)
	return acquisitions, err
}

func (s *imageCatcherServiceImpl) GetTiles(tileFilter *models.TileFilter) ([]*models.TemImageROI, error) {
	session, _ := s.dbHandler.OpenSession(true)
	temImages, err := dao.RetrieveTemImages(tileFilter, session)
	session.Close(err)

	return temImages, err
}

func (s *imageCatcherServiceImpl) RetrieveAcquisitionFile(acqMosaic *models.TemImageMosaic, path string) (jfsContent []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Unexpcted error while retrieving file for %v", acqMosaic)
		}
	}()
	fileService, err := s.getAcqFileService(&acqMosaic.Acquisition)
	if err != nil {
		return jfsContent, err
	}
	if jfsContent, err = fileService.Get(path); err != nil {
		return jfsContent, fmt.Errorf("Error retrieving content from JFS for %s: %v", path, err)
	}
	return jfsContent, nil
}

func (s *imageCatcherServiceImpl) GetProjects(filter *models.ProjectFilter) ([]*models.Project, error) {
	session, _ := s.dbHandler.OpenSession(true)
	projects, err := dao.RetrieveDistinctAcquisitionProjects(filter, session)
	session.Close(err)
	return projects, err
}

func (s *imageCatcherServiceImpl) Ping() error {
	if err := s.dbHandler.Ping(); err != nil {
		return err
	}
	if err := s.pingJfs(); err != nil {
		return err
	}
	return nil
}

func (s *imageCatcherServiceImpl) pingJfs() error {
	data := map[string]string{}
	jfsPingContent := []byte("JFS Ping")
	okchan := make(chan struct{})
	errschan := make(chan error)
	go func() {
		fileService, err := s.getFileService(s.jfsMapping["PING"])
		if err != nil {
			logger.Errorf("Error opening a file service for ping: %v", err)
			errschan <- err
			return
		}
		_, werr := fileService.Put(s.getJfsFilePath(0, jfsPingFileType), jfsPingContent, data)
		// if there was a write error the collection may be an immutable collection so we still want
		// to attempt a read to see if this succeeds
		if werr != nil {
			logger.Errorf("Error sending the ping message: %v", werr)
		}
		rerr := fileService.Verify(s.getJfsFilePath(0, jfsPingFileType), data)
		if rerr != nil {
			logger.Errorf("Error verifying the ping message: %v", werr)
			errschan <- rerr
		} else {
			var ok struct{}
			okchan <- ok
		}
		return
	}()
	select {
	case err := <-errschan:
		logger.Errorf("Error sending data to JFS: %v", err)
		return err
	case <-okchan:
		break
	case <-time.After(1.5 * 1e9):
		logger.Errorf("JFS/Scality write timeout")
		return fmt.Errorf("JFS/Scality write timeout")
	}
	return nil
}
