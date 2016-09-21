#!/bin/bash

WORKING_DIR=`pwd -P`

${WORKING_DIR}/catchersimulator $*

mv /scratch/goinac/* ${WORKING_DIR}/logs
