#!/bin/sh

acqService="localhost:5001"
acqDir="/tier2/flyTEM/data/FAFB00/1512/151215051552_14x16"

usage="$0 [-as <acq service ($acqService)>] -ad <acquisition directory> [-n]"
runner=""
while [[ $# > 0 ]]; do
    key="$1"
    if [ "$key" == "" ] ; then
	break
    fi
    shift # past the key
    case $key in
	-as|--acq-service-url)
	    acqService=$1
	    shift # past value
	    ;;
	-ad|--acq-dir)
	    acqDir=$1
	    shift # past value
	    ;;
	-n)
	    runner="echo"
	    ;;
	-t)
	    runner="time"
	    ;;
	-h|--help)
	    echo "$usage"
	    exit 0
	    ;;
	*)
	    # unknown option
	    echo "$usage"
	    exit 1
	    ;;
    esac
done

acqName=`basename $acqDir`
acqId=`echo $acqName | sed -e "s/_.*//"`

acqLog=`ls $acqDir/*_logfile.ini`
acqRoiTiles=`ls $acqDir/*_ROI_tiles.csv`
acqRoiSpec=`ls $acqDir/*_ROI_spec.ini`
acqMosaic=`ls $acqDir/*_mosaic.png`

$runner curl -i -X POST http://$acqService/service/v1/start-acquisition -F acq-inilog=@"$acqLog"

$runner curl -i -X POST http://$acqService/service/v1/create-rois/$acqId -F roi-spec=@"$acqRoiSpec" -F roi-tiles=@"$acqRoiTiles"

tileDirs=`ls $acqDir | grep col`

tileCounter=0
for tileDir in $tileDirs; do
    echo "Submit tiles from $acqDir/$tileDir"
    for tileFile in `ls $acqDir/$tileDir`; do
	echo "Submit tile file $acqDir/$tileDir/$tileFile"
	$runner curl -X PUT http://$acqService/service/v1/capture-image-content/$acqId?tile-filename=$tileFile --data-binary @"$acqDir/$tileDir/$tileFile"
	let tileCounter++
    done
done

$runner curl -i -X POST http://$acqService/service/v1/end-acquisition/$acqId -F ancillary-file=@$acqMosaic

echo "Submitted $tileCounter image tiles for $acqId"
