package dao

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"time"

	"imagecatcher/config"
	"imagecatcher/diagnostic"
	"imagecatcher/logger"
)

const maxRows = 1000

// NullableTime nullable SQL timestamp data type
type NullableTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullableTime) Scan(value interface{}) (err error) {
	nt.Valid = false
	if value == nil {
		var zTime time.Time
		nt.Time = zTime
		return nil
	}
	svalue := string(value.([]byte))
	if nt.Time, err = time.Parse("2006-01-02 15:04:05", svalue); err == nil {
		nt.Valid = true
	}
	return
}

// Value implements the driver Valuer interface.
func (nt NullableTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

// DbHandler is the database session manager
type DbHandler interface {
	diagnostic.Pingable
	OpenSession(autoCommit bool) (DbSession, error)
}

// DbSession is the database session
type DbSession interface {
	selectQuery(sqlQuery string, args ...interface{}) (*sql.Rows, error)
	execQuery(execQuery string, args ...interface{}) (sql.Result, error)
	Close(err error) error
}

// NewDbHandler creates a database session manager for the given database.
func NewDbHandler(config config.Config) (DbHandler, error) {
	dbUser := config.GetStringProperty("DB_USER", "")
	dbPassword := config.GetStringProperty("DB_PASSWORD", "")
	dbHost := config.GetStringProperty("DB_HOST", "")
	dbName := config.GetStringProperty("DB_NAME", "")

	db, err := sql.Open("mysql", dbUser+":"+dbPassword+"@"+"tcp("+dbHost+")/"+dbName)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(config.GetIntProperty("MAX_DB_CONNECTIONS", 0))
	db.SetMaxIdleConns(0)

	return WrapDB(db), nil
}

// WrapDB wraps a DB in a DbHandler
func WrapDB(db *sql.DB) DbHandler {
	return dbHandlerImpl{db: db}
}

type dbHandlerImpl struct {
	db *sql.DB
}

func (dbh dbHandlerImpl) OpenSession(autoCommit bool) (DbSession, error) {
	if autoCommit {
		return autoCommitDbSession{dbh.db}, nil
	}
	tx, err := dbh.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("Cannot start DB transaction %v", err)
	}
	return txDbSession{tx: tx}, err
}

func (dbh dbHandlerImpl) Ping() error {
	s, err := dbh.OpenSession(true)
	if err != nil {
		return fmt.Errorf("Error opening a database session: %v", err)
	}
	rows, err := s.selectQuery("select count(1) from tem_camera_images")
	defer closeRS(rows)
	if err != nil {
		s.Close(err)
		return fmt.Errorf("Error accessing the database %v", err)
	}
	if !rows.Next() {
		err = fmt.Errorf("tem_camera_images.count should return one row")
		s.Close(err)
		return err
	}
	var count int
	err = rows.Scan(&count)
	if err != nil {
		s.Close(err)
		return fmt.Errorf("Error extracting tem_camera_images.count from the result set: %v", err)
	}

	s.Close(nil)
	return nil
}

type autoCommitDbSession struct {
	db *sql.DB
}

func (s autoCommitDbSession) selectQuery(sqlQuery string, args ...interface{}) (*sql.Rows, error) {
	pstmt, err := s.db.Prepare(sqlQuery)
	if err != nil {
		logDbError("Error %v while executing %s. ", sqlQuery, err, args...)
		return nil, err
	}
	defer pstmt.Close()
	rows, err := pstmt.Query(args...)
	if err != nil {
		logDbError("Error %v while executing %s. ", sqlQuery, err, args...)
		return rows, err
	}
	return rows, err
}

func (s autoCommitDbSession) execQuery(execQuery string, args ...interface{}) (res sql.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error executing %s: %v", execQuery, r)
		}
	}()
	pstmt, err := s.db.Prepare(execQuery)
	if err != nil {
		logDbError("Error %v while executing %s. ", execQuery, err, args...)
		return nil, err
	}
	defer closeStmt(pstmt)
	res, err = pstmt.Exec(args...)
	if err != nil {
		logDbError("Error %v while executing %s. ", execQuery, err, args...)
		return res, err
	}
	return res, err
}

func (s autoCommitDbSession) Close(err error) error {
	return nil
}

type txDbSession struct {
	tx *sql.Tx
}

func (s txDbSession) selectQuery(sqlQuery string, args ...interface{}) (*sql.Rows, error) {
	pstmt, err := s.tx.Prepare(sqlQuery)
	if err != nil {
		logDbError("Error %v while executing %s. ", sqlQuery, err, args...)
		return nil, err
	}
	rows, err := pstmt.Query(args...)
	if err != nil {
		logDbError("Error %v while executing %s. ", sqlQuery, err, args...)
		return rows, err
	}
	return rows, err
}

func (s txDbSession) execQuery(execQuery string, args ...interface{}) (res sql.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Error executing %s: %v", execQuery, r)
		}
	}()
	res, err = s.tx.Exec(execQuery, args...)
	if err != nil {
		logDbError("Error %v while executing %s. ", execQuery, err, args...)
		return res, err
	}
	return res, err
}

func (s txDbSession) Close(err error) error {
	var closeErr error
	if err != nil {
		closeErr = s.tx.Rollback()
		if closeErr != nil {
			logger.Errorf("Error while rolling back the transaction %v", closeErr)
		}
	} else {
		closeErr = s.tx.Commit()
		if closeErr != nil {
			logger.Errorf("Error while committing the transaction %v", closeErr)
		}
	}
	return closeErr
}

func addFlagQueryCond(queryBuffer *bytes.Buffer, condFlag bool, value interface{}, dbCond string, queryArgs []interface{}, clause string) ([]interface{}, string) {
	if condFlag {
		queryBuffer.WriteString(clause)
		queryBuffer.WriteString(dbCond)
		queryBuffer.WriteString(" ")
		if value != nil {
			switch v := value.(type) {
			case []interface{}:
				queryArgs = append(queryArgs, v...)
			default:
				queryArgs = append(queryArgs, v)
			}
		}
		clause = "and "
	}
	return queryArgs, clause
}

func logDbError(fmt, query string, err error, args ...interface{}) {
	logargs := make([]interface{}, 0, 128)
	logargs = append(logargs, err, query)
	logargs = append(logargs, args...)
	logger.Errorf(fmt, logargs...)
}

func insert(s DbSession, insertQuery string, args ...interface{}) (int64, error) {
	res, err := s.execQuery(insertQuery, args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return id, fmt.Errorf("Error extracting the ID of last insert %v", err)
	}
	return id, err
}

func update(s DbSession, updateQuery string, args ...interface{}) (int64, error) {
	res, err := s.execQuery(updateQuery, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		logDbError("Error %v while extracting the number of updated rows for %s. ", updateQuery, err, args...)
		return n, err
	}
	return n, nil
}

func closeStmt(stmt *sql.Stmt) error {
	if stmt == nil {
		return nil
	}
	err := stmt.Close()
	if err != nil {
		return fmt.Errorf("Error while closing the statement: %v", err)
	}
	return err
}

func closeRS(rs *sql.Rows) error {
	if rs == nil {
		return nil
	}
	err := rs.Close()
	if err != nil {
		return fmt.Errorf("Error while closing the result set: %v", err)
	}
	return err
}
