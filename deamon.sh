#!/bin/bash

while true
do
    echo "start daemon"
    # start iiocalibration
    curDir=$(cd $(dirname $0);pwd)
    echo $curDir
    $curDir/iiocalibration server > $curDir/iiocalibration.log 2>&1 &
    caliPid=$!
    echo "iiocalibration pid = $caliPid"

    wait $dataMixPid
    echo "iiocalibration exit unexpected, neet restart"
    sleep 10s
done

exit 1

