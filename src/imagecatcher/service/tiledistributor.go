package service

import (
	"fmt"
	"time"

	"imagecatcher/config"
	"imagecatcher/dao"
	"imagecatcher/logger"
	"imagecatcher/models"
)

// SelectTileResultType the result type for selecting and updating tile state
type SelectTileResultType int16

const (
	// NoTileReady no tile is ready yet
	NoTileReady SelectTileResultType = iota
	// TileFound a tile was selected and returned to the caller
	TileFound
	// NoTileReadyInSection this result is returned if the section is provided but not tile
	// with the specified status is found in the given section
	NoTileReadyInSection
	// ServedEntireSection this result is returned if the section is provided and
	// the other tiles in section have the new status
	ServedEntireSection
	// ServedEntireAcq this result is returned if the acquisition is provided and
	// the other tiles in the acquisition have the new status
	ServedEntireAcq
	// InvalidTileRequest is returned if some required filter parameter is not set, such as tileState
	InvalidTileRequest
)

// String SelectTileResultType string representation
func (rt SelectTileResultType) String() string {
	switch rt {
	case NoTileReady:
		return "NO_TILE_READY"
	case TileFound:
		return "TILE_FOUND"
	case NoTileReadyInSection:
		return "NO_TILE_READY_IN_SECTION"
	case ServedEntireSection:
		return "SERVED_ALL_SECTION"
	case ServedEntireAcq:
		return "SERVED_ALL_ACQ"
	default:
		return "INVALID"
	}
}

// TileDistributor is responsible with serving available tiles and batch updating the tiles state.
type TileDistributor interface {
	ServeNextAvailableTiles(tileFilter *models.TileFilter, newTileState string) ([]*models.TileSpec, SelectTileResultType, error)
	UpdateTilesState(tiles []*models.TileSpec, tileState string) (int64, error)
}

type tileDistributorImpl struct {
	dbh dao.DbHandler
	cfg config.Config
}

// NewTileDistributor - instantiante a tile distributor
func NewTileDistributor(dbHandler dao.DbHandler, config config.Config) TileDistributor {
	return &tileDistributorImpl{dbh: dbHandler, cfg: config}
}

// ServeNextAvailableTiles picks next available tiles by acquisition ID, section number and state
// and it updates the state to the 'newTileState'
func (td *tileDistributorImpl) ServeNextAvailableTiles(tileFilter *models.TileFilter, newTileState string) ([]*models.TileSpec, SelectTileResultType, error) {
	var notilespec, tilespecs []*models.TileSpec
	var err error

	if tileFilter.State == "" {
		return notilespec, InvalidTileRequest, fmt.Errorf("The current tile state must be specified")
	}
	session, err := td.dbh.OpenSession(false)
	if err != nil {
		return notilespec, NoTileReady, fmt.Errorf("Error opening a database session: %v", err)
	}
	defer func() {
		if session != nil {
			session.Close(err)
		}
	}()

	if tilespecs, err = dao.UpdateTilesStatus(tileFilter, newTileState, session); err != nil {
		return notilespec, NoTileReady, err
	}
	session.Close(nil)
	session = nil
	if len(tilespecs) > 0 {
		sectionCountMap := make(map[string]int)
		var lastSectionCountErr error
		for _, tilespec := range tilespecs {
			sampleSectionKey := fmt.Sprintf("%s^%d", tilespec.SampleName, tilespec.NominalSection)
			sectionCount, found := sectionCountMap[sampleSectionKey]
			if !found {
				if sectionCount, err = td.countPrevAcqForSection(tilespec.SampleName, tilespec.NominalSection, tilespec.Acquired); err != nil {
					lastSectionCountErr = fmt.Errorf("Error retrieving count of previous acquisitions for section %s: %d: %v", tilespec.SampleName, tilespec.NominalSection, err)
					logger.Error(lastSectionCountErr)
				} else {
					sectionCountMap[sampleSectionKey] = sectionCount
				}
			}
			tilespec.PrevAcqCount = sectionCount
			tilespec.SetTileImageURL(td.cfg.GetStringProperty("TILES_URL_BASE", ""))
			tilespec.SetMaskURL()
			tilespec.SetTransformationRefID()
		}
		return tilespecs, TileFound, lastSectionCountErr
	}
	if tileFilter.AcqUID == 0 && tileFilter.NominalSection < 0 {
		// if neither an acquisition nor a section was provided simply return NoTileReady
		return nil, NoTileReady, nil
	}
	session, _ = td.dbh.OpenSession(true)
	defer func() {
		session.Close(err)
	}()

	var acqCompleted bool
	var acq *models.Acquisition
	if tileFilter.AcqUID > 0 {
		acquisitionFilter := &models.AcquisitionFilter{
			Acquisition: tileFilter.Acquisition,
			Pagination:  models.Page{0, 1},
		}
		var acqs []*models.Acquisition
		if acqs, err = dao.RetrieveAcquisitions(acquisitionFilter, session); err != nil {
			return nil, NoTileReady, err
		}
		if len(acqs) > 0 {
			acq = acqs[0]
			acqCompleted = acq.IsCompleted()
		}
	}
	if !acqCompleted {
		// if acquisition is not completed yet return no tile ready in section or no tile ready
		// depending on whether a particular section is requested or not
		if tileFilter.NominalSection > 0 {
			return nil, NoTileReadyInSection, err
		}
		return nil, NoTileReady, err
	}
	// if acquisition is completed count how many tiles are in the new state
	// if the number of tiles in the new state equals the total then return nothing to serve for acquisition or section
	// if it doesn't than simply return no tile ready
	newTileStateFilter := *tileFilter
	newTileStateFilter.State = newTileState
	tilesInNewStateCount, totalTilesCount, err := td.countTiles(&newTileStateFilter)
	if err != nil {
		if tileFilter.NominalSection > 0 {
			return nil, NoTileReadyInSection, err
		}
		return nil, NoTileReady, err
	}
	if totalTilesCount > 0 && tilesInNewStateCount == totalTilesCount {
		if tileFilter.NominalSection > 0 {
			return nil, ServedEntireSection, err
		}
		return nil, ServedEntireAcq, err
	}
	if tileFilter.NominalSection > 0 {
		return nil, NoTileReadyInSection, nil
	}
	return nil, NoTileReady, nil
}

func (td *tileDistributorImpl) countPrevAcqForSection(sampleName string, section int64, t time.Time) (c int, err error) {
	session, _ := td.dbh.OpenSession(true)
	defer func() {
		session.Close(err)
	}()
	return dao.CountPrevAcqContainingSection(sampleName, section, t, session)
}

// UpdateTilesState updates the state for all specified tiles.
func (td *tileDistributorImpl) UpdateTilesState(tiles []*models.TileSpec, tileState string) (nUpdates int64, err error) {
	session, err := td.dbh.OpenSession(false)
	if err != nil {
		return
	}
	defer func() {
		session.Close(err)
	}()
	nUpdates, err = dao.UpdateTemImageROIState(tiles, tileState, session)
	return
}

func (td *tileDistributorImpl) countTiles(tileFilter *models.TileFilter) (tilesStateCount, totalTilesCount int, err error) {
	session, _ := td.dbh.OpenSession(true)
	countsByAcqSectionStatus, err := dao.CountTilesByStatus(tileFilter, session)
	session.Close(err)
	if tileFilter.AcqUID == 0 && tileFilter.NominalSection < 0 || err != nil {
		return
	}
	if tileFilter.AcqUID > 0 && tileFilter.NominalSection >= 0 {
		// both acquistion and section are defined
		countsBySection := countsByAcqSectionStatus[tileFilter.AcqUID]
		if len(countsBySection) == 0 {
			return
		}
		tilesStateCount, totalTilesCount = getKeyAndTotalCount(countsBySection[tileFilter.NominalSection], tileFilter.State)
		return
	}
	if tileFilter.AcqUID > 0 {
		// acquisition is defined but there's no section
		countsBySection := countsByAcqSectionStatus[tileFilter.AcqUID]
		if len(countsBySection) == 0 {
			return
		}
		// add the counts from all sections
		for _, sectionCounts := range countsBySection {
			stateCount, sectionTilesCount := getKeyAndTotalCount(sectionCounts, tileFilter.State)
			tilesStateCount += stateCount
			totalTilesCount += sectionTilesCount
		}
	}
	// section is defined so gather the section from all acquisitions
	for _, acqCounts := range countsByAcqSectionStatus {
		for currentSection, sectionCounts := range acqCounts {
			if currentSection == tileFilter.NominalSection {
				stateCount, sectionTilesCount := getKeyAndTotalCount(sectionCounts, tileFilter.State)
				tilesStateCount += stateCount
				totalTilesCount += sectionTilesCount
			}
		}
	}
	return
}

func getKeyAndTotalCount(keysCountMap map[string]int, key string) (keyCount, totalCount int) {
	if len(keysCountMap) == 0 {
		return
	}
	for k, count := range keysCountMap {
		totalCount += count
		if k == key {
			keyCount = count
		}
	}
	return
}
