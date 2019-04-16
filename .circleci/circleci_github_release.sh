#!/bin/bash

USER="iotexproject"
PROJECT="iotex-core"
BRANCH="master"
VERSION="0"
CIRCLE_TOKEN="<YOUR_ACCESS_TOKEN>"
while getopts v:b: option
do
    case "${option}"
        in
        v) VERSION="$OPTARG";;
        b) BRANCH="$OPTARG";;
    esac
done
if [ "$VERSION" = "0" ]; then
    echo "Usage: $0 -v <version> [-b <branch>]"
    exit 1
fi

CIRCLEJOBS="nightly_bin_build_docker nightly_bin_build_darwin"


for CIRCLE_JOB in ${CIRCLEJOBS} ; do
   curl -d "build_parameters[CIRCLE_JOB]=${CIRCLE_JOB}" -d "build_parameters[VERSION]=$VERSION" "https://circleci.com/api/v1.1/project/github/${USER}/${PROJECT}/tree/${BRANCH}?circle-token=${CIRCLE_TOKEN}"
done
