# Overview

This tool gets FTBFS logs from Launchpad and saves them for further analysis
and presentation.

It presents a web interface for a quick overview of the FTBFS list.

# Installation

## Prerequisites


### Go
You need to have Go installed, preferably from source and $GOROOT set correctly
See [ http://golang.org/doc/install.html ] (http://golang.org/doc/install.html)

### MongoDB

You need to have the mongodb server installed. The client part is optional
but good to have as it provides the shell for development and testing

    apt-get install mongodb-server mongodb-clients

## Install the package.

    goinstall github.com/janimo/ftbfs

This will in addition install the mgo and lpad packages which are used to access
MongoDB and Launchpad respectively. Their sources will be in $GOROOT/src/pkg/launchpad.net
and $GOROOT/src/pkg/github.com


# Usage

Running it with no arguments will print out the usage
    ftbfs

Fetch data from Launchpad and saving it in MongoDB (defaults to a localhost install)

    ftbfs -f

Update the failure cause field of each build entry based on matching string patterns

    ftbfs -u

Get the list of build failures due to a certain cause, as stored in the database

    ftbfs -c warnings

Serve a web page showing the FTBFS package list. This needs to be run from the package
source dir as the templates and static files are there.

    ftbfs -s

By default the server listens on port 9999, change that by passing -p. See the results

    firefox http://localhost:9999
