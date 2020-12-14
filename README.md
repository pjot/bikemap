# bikemap
CLI application that renders a map of Strava activities. It uses openstreetmap to figure out where to center the map from a user-supplied location.

## Installation
`go get && go build`

## Usage
### 1. Create a Download Request from Strava
* Settings > My Account > "Download or Delete Your Account" [Get Started] > #2
* You'll end up on https://www.strava.com/athlete/delete_your_account
* Request Your Archive
* You will get an export of all your Strava data in your email. This might take a while and is only possible to do once a week.
### 2. Run the program!
* `./bikemap -i <export-dir>/activities/ -o file.png -c Stockholm`
* 3 arguments are required, `--in-dir/-i`, `--out-file/-o`, `--center/-c`.

### All command line arguments
```$ ./bikemap -h
Usage:
  bikemap [OPTIONS]

Application Options:
      --height=   Height of output image (default: 500)
      --width=    Width of output image (default: 1000)
  -c, --center=   Location to center the map around
  -i, --in-dir=   Directory where the activity fields are
  -o, --out-file= Output filename

Help Options:
  -h, --help      Show this help message

