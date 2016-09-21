default: build

IMAGECAPTURE_DBHOST="localhost"
IMAGECAPTURE_DBPORT="3306"
IMAGECAPTURE_DB="image_capture"
IMAGECAPTURE_USER="image_capture"
IMAGECAPTURE_PASS="image_capture"

deps:
	go get -u github.com/DavidHuie/gomigrate
	go get -u github.com/JaneliaSciComp/janelia-file-services/JFSGolang/jfs
	go get -u github.com/hashicorp/golang-lru
	go get -u github.com/go-sql-driver/mysql
	go get -u github.com/golang/glog
	go get -u github.com/google/flatbuffers/go
	go get -u github.com/julienschmidt/httprouter
	go get -u github.com/satori/go.uuid
	go get -u github.com/vaughan0/go-ini

fmt:
	@go fmt src/cmd/imagecatcher/*.go
	@go fmt src/cmd/imagecatcherclient/*.go
	@go fmt src/cmd/catchersimulator/*.go
	@go fmt imagecatcher/dao
	@go fmt imagecatcher/diagnostic
	@go fmt imagecatcher/models
	@go fmt imagecatcher/netutils
	@go fmt imagecatcher/protocol
	@go fmt imagecatcher/service
	@go fmt imagecatcher/utils

lint:
	@golint src/cmd/imagecatcher
	@golint src/cmd/imagecatcherclient
	@golint src/cmd/catchersimulator
	@golint imagecatcher/dao
	@golint imagecatcher/diagnostic
	@golint imagecatcher/models
	@golint imagecatcher/netutils
	@golint imagecatcher/protocol
	@golint imagecatcher/service
	@golint imagecatcher/utils

test:
	@go test --race -v imagecatcher/dao
	@go test --race -v imagecatcher/daotest
	@go test --race -v imagecatcher/models
	@go test -v imagecatcher/service
	@go test --race -v imagecatcher/netutils
	@go test --race -v imagecatcher/utils

benchmarks:
	@go test -v  -benchmem -benchtime 10s -parallel 8 imagecatcher/service -bench "Benchmark.+" -run None

compileidls: src/vendor/idls/tilerequests/*.go

src/vendor/idls/tilerequests/*.go: idls/tilerequest.idl idls/tileresponse.idl
	flatc -g -o src/vendor $^

build: build-server build-client test

build-tools:
	@go build -o imagecatchertools src/cmd/imagecatcher/imagecatchertools.go

build-server: src/vendor/idls/tilerequests/*.go
	@go build -o imagecatcher src/cmd/imagecatcher/main.go

build-client: src/vendor/idls/tilerequests/*.go
	@go build -o imagecatcherclient src/cmd/imagecatcherclient/main.go
	@go build -o catchersimulator src/cmd/catchersimulator/main.go

clean:
	@rm -f catchersimulator imagecatcher imagecatcherclient imagecatchertools

drop-db:
	@mysql -h ${IMAGECAPTURE_DBHOST} \
		-u ${IMAGECAPTURE_USER} \
		-p${IMAGECAPTURE_PASS} ${IMAGECAPTURE_DB} -e "source dbscripts/init-db/dropTables.sql"

init-db:
	@mysql -h ${IMAGECAPTURE_DBHOST} \
		-u ${IMAGECAPTURE_USER} \
		-p${IMAGECAPTURE_PASS} ${IMAGECAPTURE_DB} -e "source dbscripts/init-db/createTables.sql"
	@mysql -h ${IMAGECAPTURE_DBHOST} \
		-u ${IMAGECAPTURE_USER} \
		-p${IMAGECAPTURE_PASS} ${IMAGECAPTURE_DB} -e "source dbscripts/init-db/populateTables.sql"

upgrade-db: build-tools
	@./imagecatchertools -tool db-upgrade \
		-dbhost ${IMAGECAPTURE_DBHOST}:${IMAGECAPTURE_DBPORT} \
		-dbname ${IMAGECAPTURE_DB} \
		-dbpass ${IMAGECAPTURE_PASS} \
		-dbuser ${IMAGECAPTURE_USER} \
		-migrations_dir dbscripts/migrate-db

downgrade-db: build-tools
	@./imagecatchertools -tool db-downgrade \
		-dbhost ${IMAGECAPTURE_DBHOST}:${IMAGECAPTURE_DBPORT} \
		-dbname ${IMAGECAPTURE_DB} \
		-dbpass ${IMAGECAPTURE_PASS} \
		-dbuser ${IMAGECAPTURE_USER} \
		-migrations_dir dbscripts/migrate-db

reset-db: drop-db init-db
