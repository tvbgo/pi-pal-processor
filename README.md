Hello and Welcome to the Pi-Pal-Processor

First off it must be noted that this work is derived from the
[pi-delivery](https://github.com/GoogleCloudPlatform/pi-delivery) project by authored by (IMHO the amazing)Emma Haruka Iwao.
Please refer to the License for any doubts about use, reproduction or distribuition.

## Pre-Setup

You will need to do some steps before executing this project

1. Have or create and account on GCP
2. Download gcloud cli
3. Create a bucket to store your digits and results
4. Download the digits of pi using gsutil
`gsutil -m rsync -R gs://pi100t gs://YOUR_BUCKET`
5. Create a service account and download the json
with credentials. Place it anywhere on this project
6. Have or install [Golang](https://go.dev/doc/install)

Opt:
7. Have or install Ruby to use the result processor

## Setup

1. Open your terminal and set the following ENVs:
```
export PROJECT=YOUR-GCP-PROJECT-NAME
export REGIONS=(REGION OR REGIONS FOR YOUR BUCKET)
export STAGE_BUCKET=YOUR-BUCKET-NAME
export GCF_API_SA=YOUR-SERVICE-ACCOUNT-NAME
export GOOGLE_APPLICATION_CREDENTIALS=PATH-TO-THE-JSON-DOWNLOADED-ON-PRE-SETUP
```

2. Run `go run cmd/pi-processor`

**Obs: The process also accepts the flag '-s' which is the starting index of the digit
**ex: `go run cmd/pi-processor -s 1000000`

This project already includes in the full_results directory
a list of every palindrome over 17 digits along with the start
and index of the palindrome + the 2 largest prime palindromes
in the 100 trillion digits of Pi.

The ruby script in result_processor.rb processes the results
and uses the oficial API to validate the position of the palindrome


## Warnings

1. On line 24 of main.go we define how many workers will be used for
processing, these will be processes in parallel, which means you will
need to adjust the amount according the the CPU and RAM of your machine/VM
During my processing, I found that 50 workers for e2-highmem-4(4 CPU and 16GB RAM)
and 100 workers for e2-highmem-8(8 CPU and 32GB RAM) had the best results.

2. Running this code locally seemed to incur on higher expense on GCP
than running on a provisioned VM
