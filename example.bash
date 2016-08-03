#!/bin/bash

function process {
  for x in {1..100}
  do
    echo "$1 `date`"
    sleep $x
  done
}

process "($*) STDOUT" &
process "($*) STDERR" 1>&2 &

sleep 3600
