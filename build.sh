#!/bin/sh

if [ -z "$1" ]
  then
    echo "No argument supplied"
fi

STATE=$1

#*****************************************************************
#************************ Building binary ******************
#*****************************************************************
echo "Attempting to build..."
go version
env GOOS=linux go build -ldflags="-s -w" -o bin/covid-vaccine-$STATE main.go

zip covid-vaccine-$STATE.zip bin/covid-vaccine-$STATE

aws lambda update-function-code \
    --function-name  covid-vaccine-$STATE \
    --zip-file fileb://./covid-vaccine-$STATE.zip