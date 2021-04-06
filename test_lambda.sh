#!/bin/sh

if [ -z "$1" ]
  then
    echo "No argument supplied"
fi

STATE=$1

aws --region us-east-1 lambda invoke \
--function-name covid-vaccine-$STATE \
text.json
