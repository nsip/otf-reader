#!/bin/bash


# iterate all json files in this folder
# run jq to restructure json records
# save result with .brightpath extension
FILES=./*.json
for f in $FILES
do
  echo "Processing $f file..."
    jq \
    'map([.allocations[] + {school: .school} + {test: {name: .name, date_administered: .date_administered, termoccurrence: .termocurrence, description: .description, years: .years, scale: .scale, assessment_type: .assessment_type }}])|flatten' \
    $f > $f.brightpath
done

