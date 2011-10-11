// A tool that lists ARM packages that FTBFS due to builder timeout

package main

import (
	"os"
	"fmt"
	"http"
	"flag"
	"strings"
	"io/ioutil"
	"launchpad.net/gobson/bson"
	"launchpad.net/mgo"
	"launchpad.net/lpad"
)

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

//Login to Launchpad
func login() lpad.Root {
	root, err := lpad.Login(lpad.Production, &NoAuth{})
	check(err)
	return root
}

//Build error messages should only be looked for in the lastBytes of the log text
const lastBytes = 20 * 1024

//Retrieve the tail of the contents of a buildlog text pointed to by the URL

func getBuildLog(url string) string {
	response, err := http.Get(url)
	check(err)
	defer response.Body.Close()

	b, err := ioutil.ReadAll(response.Body)
	check(err)

	e := len(b) - 1
	s := 0
	if e > lastBytes {
		s = e - lastBytes
	}
	return string(b[s:e])
}

//Check if a build log is in the database
func stored(url string) bool {
	q := collection.Find(bson.M{"url":url})
	c, err := q.Count()
	check(err)
	return c > 0
}

func store(url, content, datecreated string) {
	collection.Insert(bson.M{"url":url, "content":content, "datecreated":datecreated})
}

const timeoutMessage = "Build killed with signal 15"

var ftbfs_list map[string]string

func process(b lpad.Build, state lpad.BuildState) {

	url := b.BuildLogURL()

	if !strings.Contains(url, "armel") {
		return
	}

	if stored(url) {
		return
	}

	fmt.Println("Checking", url)
	content := getBuildLog(url)

	if strings.Contains(content, timeoutMessage) {
		ftbfs_list[url] = b.DateCreated()
	}

	store(url, content, b.DateCreated())
}

//Find current FTBFS logs.

func ftbfs(root lpad.Root, source_name string) {
	ubuntu, _ := root.Distro("ubuntu")
	series, _ := ubuntu.FocusSeries()
	fmt.Println("Generating ARM FTBFS list for ", series.FullSeriesName())

	ftbfs_list = make(map[string]string)
	for _, build_state := range []lpad.BuildState{lpad.BSFailedToBuild} {
		ftbfs, _ := series.GetBuildRecords(build_state, lpad.PocketRelease, source_name)
		ftbfs.For(func(b lpad.Build) os.Error {
			process(b, build_state)
			return nil
		})
	}

	fmt.Println("Timed out builds")
	fmt.Println("================")
	for url, date := range ftbfs_list {
		fmt.Println(url, date)
	}
}

const (
	MONGO_URL = "localhost"
	MONGO_DB = "FTBFS"
	MONGO_COL = "ftbfs"
)

var collection mgo.Collection

func mongoConnect() {
	fmt.Println("Connecting to MongoDB server at", MONGO_URL)
	session, err := mgo.Mongo(MONGO_URL)
	check(err)

	collection = session.DB(MONGO_DB).C(MONGO_COL)
}

func main() {
	flag.Parse()
	args := flag.Args()
	source_name := ""
	if len(args) == 1 {
		source_name = args[0]
	}

	mongoConnect()
	root := login()
	ftbfs(root, source_name)
}
