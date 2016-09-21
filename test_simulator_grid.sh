#!/bin/bash
export QSUB="qsub -A flyTEM -pe batch 16 -l sandy=true -wd $PWD -b y -e /scratch/goinac -o /scratch/goinac"

sh test_simulator.sh $*
