package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/DavidHuie/gomigrate"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL driver

	"imagecatcher/dao"
	"imagecatcher/service"
)

// Tool - action or selected tool
type Tool int

const (
	// None - no tool selected
	None Tool = iota
	// DbDowngrade - database downgrade tool
	DbDowngrade
	// DbUpgrade - database upgrade tool
	DbUpgrade
	// ImportCalibration - imports a calibration
	ImportCalibration
)

// String representation
func (t Tool) String() string {
	switch t {
	case None:
		return "none"
	case DbDowngrade:
		return "db-downgrade"
	case DbUpgrade:
		return "db-upgrade"
	case ImportCalibration:
		return "import-calibration"
	default:
		panic("Invalid tool type")
	}
}

// Set the tool value - Tool implements a flag.Value Set method
func (t *Tool) Set(value string) (err error) {
	switch strings.ToLower(value) {
	case "", "none":
		*t = None
	case "db-downgrade":
		*t = DbDowngrade
	case "db-upgrade":
		*t = DbUpgrade
	case "import-calibration":
		*t = ImportCalibration
	default:
		err = fmt.Errorf("Invalid tool value: %s - valid values are: {none, db-downgrade, db-upgrade, import-calibration}", value)
	}
	return
}

var (
	dbHost = flag.String("dbhost", "localhost:3306", "Database host")
	dbName = flag.String("dbname", "image_capture", "Database name")
	dbUser = flag.String("dbuser", "image_capture", "Database user")
	dbPass = flag.String("dbpass", "image_capture", "Database password")
)

func main() {
	var tool Tool
	flag.Var(&tool, "tool", "Tool to be used")
	migrationsPath := flag.String("migrations_dir", "./dbscripts/migrate-db", "Path to the directory containing the migrations")
	nDbMigrations := flag.Int("n", 1, "How many database migrations to apply")
	calibrationFile := flag.String("calibration", "", "Calibration file")
	maskRoot := flag.String("mask_root", "", "Masks root")

	flag.Parse()

	db, err := sql.Open("mysql", *dbUser+":"+*dbPass+"@"+"tcp("+*dbHost+")/"+*dbName)
	if err != nil {
		fmt.Println("Error initializing the database", err)
		os.Exit(1)
	}

	var toolInvocation func()
	switch tool {
	case DbDowngrade, DbUpgrade:
		toolInvocation = func() {
			migrateDb(tool, db, *migrationsPath, *nDbMigrations)
		}
	case ImportCalibration:
		toolInvocation = func() {
			importCalibration(db, *calibrationFile, *maskRoot)
		}
	default:
		fmt.Println("No tool has been specified")
		os.Exit(1)
	}
	toolInvocation()
}

func migrateDb(dbMigrateTool Tool, db *sql.DB, migrationsPath string, nDbMigrations int) {
	dbmigrator, err := gomigrate.NewMigrator(db, gomigrate.Mysql{}, migrationsPath)
	if err != nil {
		fmt.Println("Error initializing the migrator", err)
		os.Exit(1)
	}
	var migrateMethod func() error
	switch dbMigrateTool {
	case DbDowngrade:
		migrateMethod = dbmigrator.Rollback
	case DbUpgrade:
		migrateMethod = dbmigrator.Migrate
	}
	for i := 0; i < nDbMigrations; i++ {
		if err = migrateMethod(); err != nil {
			fmt.Println("Error running the migration", err)
			break
		}
	}
}

func importCalibration(db *sql.DB, calibrationFile string, maskRootURL string) {
	cf, err := os.Open(calibrationFile)
	if err != nil {
		fmt.Printf("Error opening calibration file %s: %v", calibrationFile, err)
		os.Exit(1)
	}

	var cr service.CalibrationJSONReader
	calibrations, err := cr.Read(cf)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	configurator := service.NewConfigurator(dao.WrapDB(db))
	err = configurator.ImportCalibrations(calibrations)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if maskRootURL != "" {
		for _, c := range calibrations {
			if err = configurator.UpdateMaskURL(c.TemcaID, c.Camera, maskRootURL); err != nil {
				fmt.Println(err)
			}
		}
	}
}
