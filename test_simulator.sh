#!/bin/bash

ACQ_SERVICE_URL=http://localhost:5001
TILE_SERVICE_URL=localhost:5002
TOTAL_RUNNING_TIME=30
RUNNING_PERIOD=60
BREAK_PERIOD=.25
IMAGE_SIZE_1=20m
IMAGE_SIZE_8=5.3m
REQUEST_PROCESSING_TIME_1=250
REQUEST_PROCESSING_TIME_8=150
SEND_METHOD=TCP-PUT
LOGDIR=/scratch/goinac
CONCURRENTLY="-concurrently"

JOB_RUNNER=""
while [[ $# > 0 ]]; do
    key="$1"
    if [ "$key" == "" ] ; then
	break
    fi
    shift # past the key
    case $key in
	-as|--acq-service-url)
	    ACQ_SERVICE_URL=$1
	    shift # past value
	    ;;
	-s|--service-url)
	    TILE_SERVICE_URL=$1
	    shift # past value
	    ;;
	-t|--total-running-time)
	    TOTAL_RUNNING_TIME=$1
	    shift # past value
	    ;;
	-r|--running-period-time)
	    RUNNING_PERIOD=$1
	    shift # past value
	    ;;
	-b|--break-time)
	    BREAK_PERIOD=$1
	    shift # past value
	    ;;
	-is1|--image-size-1)
	    IMAGE_SIZE_1=$1
	    shift # past value
	    ;;
	-is8|--image-size-8)
	    IMAGE_SIZE_8=$1
	    shift # past value
	    ;;
	-pt1|--processing-time-1)
	    REQUEST_PROCESSING_TIME_1=$1
	    shift # past value
	    ;;
	-pt8|--processing-time-8)
	    REQUEST_PROCESSING_TIME_8=$1
	    shift # past value
	    ;;
	-ld|--logdir)
	    LOGDIR=$1
	    shift # past value
	    ;;
	-sm|--send-method)
	    SEND_METHOD=$1
	    shift # past value
	    ;;
	-sc-|--do-send-concurrently)
	    CONCURRENTLY=""
	    ;;
	-n)
	    JOB_RUNNER="echo "
	    ;;
	-h|--help)
	    echo "$0 [-s <service url>] [-t <total running time in mins (5)>] [-r <running period in mins (.5)>] [-b <breaking period in mins (.25)>] [-is8 <image size (5.3m)>] [-pt8 <request processing time (150ms)>] [-is1 <image size (20m)>] [-pt1 <request processing time (250ms)>] [-ld <log directory>] [-sc-] [-h] [-n]"
	    exit 0
	    ;;
	*)
	    # unknown option
	    echo "$0 [-s <service url>] [-t <total running time in mins (5)>] [-r <running period in mins (.5)>] [-b <breaking period in mins (.25)>] [-is8 <image size (5.3m)>] [-pt8 <request processing time (150ms)>] [-is1 <image size (20m)>] [-pt1 <request processing time (250ms)>] [-ld <log directory>] [-sc-] [-h] [-n]"
	    exit 1
	    ;;
    esac
done

echo "ACQ SERVICE URL $ACQ_SERVICE_URL"
echo "TILE SERVICE URL $TILE_SERVICE_URL"
echo "TOTAL_RUNNING_TIME $TOTAL_RUNNING_TIME"
echo "RUNNING PERIOD $RUNNING_PERIOD"
echo "BREAK_PERIOD $BREAK_PERIOD"
echo "IMAGE_SIZE_1 ${IMAGE_SIZE_1}"
echo "IMAGE_SIZE_8 ${IMAGE_SIZE_8}"
echo "REQUEST_PROCESSING_TIME_1 ${REQUEST_PROCESSING_TIME_1}"
echo "REQUEST_PROCESSING_TIME_8 ${REQUEST_PROCESSING_TIME_8}"
echo "LOGDIR ${LOGDIR}"

SIMULATOR="${PWD}/simulator.sh"

JOB_RUNNER="${JOB_RUNNER}${QSUB} "

# Create the acquisition(s)
${JOB_RUNNER}${SIMULATOR} -action send-acq-log -service-url ${ACQ_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1502/150209140111_124x120

${JOB_RUNNER}${SIMULATOR} -action send-acq-log -service-url ${ACQ_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1503/150304104643_108x152

${JOB_RUNNER}${SIMULATOR} -action send-acq-log -service-url ${ACQ_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1504/150407200041_40x168

# Send tiles
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1504/150407200041_40x168 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 0 -image-size ${IMAGE_SIZE_1} -rate ${REQUEST_PROCESSING_TIME_1} -logfile "${LOGDIR}/cam0_20m.log" ${CONCURRENTLY} &

${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1502/150209140111_124x120 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 0 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam0_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1502/150209140111_124x120 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 1 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam1_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1502/150209140111_124x120 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 2 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam2_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1502/150209140111_124x120 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 3 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam3_5m.log" ${CONCURRENTLY} &

${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1503/150304104643_108x152 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 0 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam5_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1503/150304104643_108x152 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 1 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam6_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1503/150304104643_108x152 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 2 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam7_5m.log" ${CONCURRENTLY} &
${JOB_RUNNER}${SIMULATOR} -action send-tile -send-method ${SEND_METHOD} -service-url ${TILE_SERVICE_URL} -acq-dir-url file:///tier2/flyTEM/data/FAFB00/1503/150304104643_108x152 -total-running-time ${TOTAL_RUNNING_TIME} -running-period-time ${RUNNING_PERIOD} -break-time ${BREAK_PERIOD} -camera 3 -image-size ${IMAGE_SIZE_8} -rate ${REQUEST_PROCESSING_TIME_8} -logfile "${LOGDIR}/cam8_5m.log" ${CONCURRENTLY} &
