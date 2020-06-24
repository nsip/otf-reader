# otf-reader
The otf-reader reads assessment data in various formats from filesystem, sends into OTF progress data management workflows.

## how it works
The otf-reader is configured to watch a file folder. 

The file folder will receive output files produced by a variety of assessment systems.

Whenever files are created or updated within the folder they are read and transformed into a series of individual records that are posted to a nats stream, from where they will be consumed by the OTF Progress Data Management workflow.

Typical input files would be a csv or json file containing multiple assessment results for a cohort of students.

The reader creates a standard json record for each result read from the file, and adds meta-data about to assist the further processing of the records as they traverse the OTF PDM workflow. For example the otf-reader will add a 'providerName:' field to the created record that identifies the system that created the original record - this can be used to control conditional processing later in the workflow if necessary.

The otf-reader can monitor trees of folders recursively, and can be configured to consume only specific file types.

All configuration options can be set on the command-line using flags, via envronment variables, or by using a configuration file.
Configuration can use any or all of these methods in combination.
For example options such as the address and hostname of the nats streaming server might best be accessed from environment variables, whilst the selection of which folder to monitor might be supplied in a json configuration file.

Configuration flags are capitalised and prefixed with OTF_RDR when supplied as environment variables; so flag --natsPort on the commnad-line becomes 

```
OTF_RDR_NATSPORT=4222
```

when expressed as an environment variable and

```
{ "natsPort":4222 }
```

when set in a json configuration file.

## running otf-reader
simply launch the otf-reader with configuration options, for example:
```
./otf-reader -config=./config/lpofa_config.json
```

the binary can also be run as:
```
./otf-reader --help 
```

to display all configuration options

## configuration

These are the confiuration options:

|Option name|Type|Required|Default|Description|
|---|---|---|---|---|
|readerName|string|no|auto-generated|A unique name for this reader, added to messages to identify origin in workflows/audits. If not supplied will default to a short hashid style id|
|readerID|guid (string)|no|auto-generated|Assigns a unique id to this reader, agin used for tracing/auditing. If not supplied will default to a nuid style guid|
|providerName|string|yes||Name of the system which created the original input data|
|inputFormat|string|yes|csv|The internal format of the input data file, currently mst be one of csv or json|
|alignMethod|string|yes||Method to be applied later in workflow to align data from this provider to the NLPs, cmust be one of prescribed|mapped|inferred)|
|levelMethod|string|yes||Method to be applied later in workflow to scale data from this provider to the NLP scaling, cmust be one of prescribed|mapped-scale|rules)|
|natsPort|int|yes|4222|The port of the nats server that will receive records|
|natsHost|string|yes|localhost|The hostname/address of the nats server|
|natsCluster|string|yes|test-cluster|nats streaming cluster name|
|topic|string|yes||The name of the nats topic to publish the ingested messages to. Topics can be delimited using '.' characters. For example the provided sample configs publish to "otf.ingest"|
|config|string|no||location of a configuraiton file in json format|
|folder|string|yes|cwd|The folder that the reader should watch for file activity|
|fileSuffix|string|no||Optional filter of files based on suffix, for instance if a folder contains multiple file types but only .csv files are of interest then the watcher list can be filtered by providing this option. If not provided all files in the watched folder will be read. The file suffix does not affect the inputFormat, so that files can have any extension such as .myAssessmentApp, but still be processed as csv or json files|
|interval|string|yes|500ms|Frequecy of watcher poll interval. Should be supplied as a duriation such as 2s, 2m30s, 1h30m etc.|
|recursive|boolean|yes|true|Watches all sub-folders of the specified watcher folder for file changes, set to false will monitor the watcher folder only|
|dotFiles|boolean|yes|false|On unix systems includes dot files in monitoring for activity|
|ignore|string|no||Provide a comma-separated list of paths to ignore/exclude from watching|
|concurrFiles|int|yes|10|Number of input files to process concurrently, can be set much higher on unix systems where file-handles are not an issue|

## otf usage scenario

This repository contains all supporting files to demonstrate the initial ingest phase of the OTF PDM workflow.

The otf-reader needs an instance of nats-streaming-server to be running in order to have somewhere to publish the data to, so start an instance.

build the main binary from the /cmd/otf-reader folder

```
go get github.com/nsip/otf-reader
cd go/src/github.com/nsip/otf-reader/cmd/otf-reader
go build
```


create an input folder tree under the /cmd/otf-reader folder:

```
/in
  /brightpath
  /lpofa
  /maths-pathway
  /spa
```


to test all 4 input formats create 4 terminal sessions and then launch an istance of otf-reader in each one using the provided configs from the /cmd/otf-reader/config folder:

for lpofa xapi format data:
```
./otf-reader -config=./config/lpofa_config.json
```

for maths-pathway format data: 
```
./otf-reader -config=./config/mp_config.json
```

for sreams format data:
```
./otf-reader -config=./config/spa_config.json
```

for brightpath format data:
```
./otf-reader -config=./config/bp_config.json
```

As each reader start up it will print its configuration to the terminal and then enter the watching looop waiting for file activity.

You can now copy files from the 
```
/cmd/otf-reader/test-data
```

folder into the relevant folder under the cmd/otf-reader/in root you created earlier. 

As each file is copied, the reader will report progress in publishing the records.

The otf-reader will report to the console any files that are deleted from the watched folder for information.
The reader does maintain a checksum so that if the same file is copied under the same name with the same content into the watched folder it is not processed again.

## supporting components

### preprocessors

Data is provided to the OTF PDM workflow as a stream of individual records per student.

CSV records are typically in this format conceptually. JSON records, however can be normalised to a greater or lesser extent.

For example our sample BrightPath data needs some elements from the original file strucure such as the school information to be repeated in each record passed into the OTF. In the orginal file the school information is recorded only once for efficiency.

Therefore sometimes some pre-processing of the input data is required. To achieve the desired structure in the BrighPath data a shell script is provided in the /preprocesors/brightpath folder to de-normalise the data by using the jq json processor.

Running this script on the original BrightPath.json file creates a new BrightPath.json.brightpath file. This is the file that should be used in testing the brightpath reader, and the appropriate example config has been updated to look for .brightpath files rather than .json files accordingly.

If the original file is read, and the brightpath otf-reader has its file suffix set back to .json, no dire consequences happen - any json content can be read by the otf-reader and will always be stored in the { "original": } member of the created otf-message.
The diffrerence is that the original format file will be parsed as only containing 2 records (as the records are descended from the 2 schools in the file). The pre-processed version of the file produces 24 individual student records.

## benthos

The otf-reader is designed to be very high performance. It will throw errors immediately if no nats server is available, but assuming that a nats server is available it will publish ingested records very quickly.

Without a nats client however it's hard to validate that the messages have arrived and contain the expected reuslts.

Given that the rest of the OTF DPM workflow is handled by benthos, testing the success of the publising with a benthos workflow config kills 2 birds with one stone...

You need to have a copy of benthos installed.

Once you do, you can run it with either of the configs found in

```
/otf-reader/cmd/benthos
```

### validate-publish-multi-files

```
benthos -c ./validate-publish-multi-files.yaml
```

will start a benthos workflow that consumes the newly created otf messages from the publishing stream and writes each one as a seaparate json file
in the msgs sub-folder

```
/otf-reader/cmd/benthos/msgs
```

each message can be inspected to validate the content.

### validate-publish-single-file

```
benthos -c ./validate-publish-single-file.yaml
```

will start a benthos workflow that consumes the newly created otf messages from the publishing stream and writes each one as a seaparate line in a
consolidated log file:

```
/otf-reader/cmd/benthos/digest/msglog.txt
```


The benthos process can also be started before the otf-readers at any time to check end-to-end latency between reading files and producing output.


## output data (otf messsage)

At this stage of the OTF PDM workflow, the otf message simply contains two blocks of data, a "meta:" section with the parameters provided by the otf-reader and an "original:" block containing the content of the read data in json format. Example:

```
{
    "meta":
    {
        "providerName": "BrightPath",
        "inputFormat": "json",
        "alignMethod": "mapped",
        "levelMethod": "prescribed",
        "readerName": "yqWa7N",
        "readerID": "27L25FGqxGFnr5NxkyFUyR"
    },
    "original":
    {
        "assessor_participation":
        {
            "person":
            {
                "first_name": "Sabine",
                "last_name": "Hoffmann",
                "identifiers": [],
                "user":
                {
                    "email": "sabinehoffmann835@gmail.com"
                }
            },
            "research": false
        },
        "student_participation":
        {
            "name": "",
            "hard_copy": true,
            "work": null,
            "enrolment":
            {
                "student":
                {
                    "first_name": "Kandra",
                    "last_name": "Bosarge",
                    "identifiers": [
                    {
                        "identifier": "187",
                        "provider": "Research Student ID"
                    }],
                    "gender": 2,
                    "dob": "2013-11-21",
                    "atsi": true,
                    "lbote": null,
                    "eald": null
                },
                "cohort":
                {
                    "calendar_year": 2020,
                    "academic_year": 3
                }
            }
        },
        "score": 350,
        "notes": "",
        "parent_comments": "<p>Teacher Comments</p>",
        "student_comments": "<ul>\n<li>Individual teacher comment</li>\n</ul>",
        "descriptor": "<ul>\n<li>Individual teacher comment - what you did well.</li>\n</ul>",
        "teaching_points": "<ul>\n<li>Individual teacher comment -&nbsp;start to</li>\n</ul>",
        "additional_schools": [],
        "school":
        {
            "name": "WA Demo School",
            "identifiers": [
            {
                "identifier": "1Demo",
                "provider": "Research School ID"
            }],
            "sector": "Research Sector"
        },
        "test":
        {
            "name": "Test project for NSIP",
            "date_administered": "2020-06-02",
            "termoccurrence": null,
            "description": "Test data for NSIP",
            "years": [
                "Year 1",
                "Year 2"
            ],
            "scale": "Narrative Scale",
            "assessment_type": "SINGLE_ASSESSOR"
        }
    }
}
```







