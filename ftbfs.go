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
const lastBytes = 100 * 1024

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

//Has a value for all architectures we care about, currently only ARM
var archs = map[string]bool{"armel":true}

func process(b lpad.Build, state lpad.BuildState) {

	//Ignore architectures we do not care about
	if !archs[b.ArchTag()] {
		return
	}

	//Ignore builds with no SPPH as they are not the most recent
	spph, err := b.CurrentSourcePublicationLink()
	if err != nil {
		return
	}

	save(b, spph)
}

//Check if a build log is in the database
func stored(url string) bool {
	q := collection.Find(bson.M{"url":url})
	c, err := q.Count()
	check(err)
	return c > 0
}

var errorPatterns = map[string]string{ "timeout":"Build killed with signal 15" }

func save(b lpad.Build, spph lpad.SPPH) {

	url := b.BuildLogURL()

	if stored(url) {
		return
	}

	content := getBuildLog(url)

	cause := "other"

	for c, p := range errorPatterns {
		if strings.Contains(content, p) {
			cause = c
			break
		}
	}

	fmt.Printf("Saving error log for %s %s %s", spph.PackageName(), spph.PackageVersion(), b.ArchTag())

	collection.Insert(bson.M{"url":url, "cause": cause, "content":content, "datecreated":b.DateCreated()})
}

//Find current FTBFS logs
func getFTBFS(root lpad.Root, source_name string) {
	ubuntu, _ := root.Distro("ubuntu")
	series, _ := ubuntu.FocusSeries()
	fmt.Println("Generating FTBFS list for", series.FullSeriesName())

	for _, build_state := range []lpad.BuildState{lpad.BSFailedToBuild} {
		ftbfs, _ := series.GetBuildRecords(build_state, lpad.PocketRelease, source_name)
		ftbfs.For(func(b lpad.Build) os.Error {
			process(b, build_state)
			return nil
		})
	}
}

const (
	MONGO_URL = "localhost"
	MONGO_DB = "FTBFS"
	MONGO_COL = "ftbfs"
)

var collection mgo.Collection

func mongoConnect() {
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
	getFTBFS(root, source_name)
}
