package daotest

import (
	"log"

	"imagecatcher/config"
	"imagecatcher/dao"
)

const (
	testDBUser     = "image_capture"
	testDBPassword = "image_capture"
	testDBHost     = "localhost:3306"
	testDBName     = "image_capture_test"
)

var TestDBHandler dao.DbHandler

func InitTestDB() {
	if TestDBHandler == nil {
		var err error
		TestDBHandler, err = dao.NewDbHandler(config.Config{
			"DB_USER":            testDBUser,
			"DB_PASSWORD":        testDBPassword,
			"DB_HOST":            testDBHost,
			"DB_NAME":            testDBName,
			"MAX_DB_CONNECTIONS": 0,
		})
		if err != nil {
			log.Panic(err)
		}
	}
}
