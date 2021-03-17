#!/bin/sh

aws --region us-east-1 lambda invoke \
--function-name covid-vaccine-iowa \
text.json
