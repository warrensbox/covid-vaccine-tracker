#!/bin/sh

echo "hello"

#*****************************************************************
#************************ Building binary ******************
#*****************************************************************

go version
# echo $GOCACHE
# export GOCACHE=cache
env GOOS=linux go build -ldflags="-s -w" -o bin/covid-vaccine-iowa main.go

zip covid-vaccine-iowa.zip bin/covid-vaccine-iowa

aws lambda update-function-code \
    --function-name  covid-vaccine-iowa \
    --zip-file fileb://./covid-vaccine-iowa.zip