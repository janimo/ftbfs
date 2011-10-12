package main

import (
	"os"
	"fmt"
	"http"
	"flag"
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

//A set of hardcoded matches. Needs to allow user specified patterns to be more useful and generic
var errorPatterns = map[string]string{
											"timeout" : "Build killed with signal 15",
											"segfault" : "Segmentation fault",
											"gcc-ice" : "internal compiler error",
											"cmake" : "CMake Error",
											"opengl" : "error: '(GL|gl).* was not declared",
											"qtopengl": "error: 'GLdouble' has a previous declaration",
											"linker": "final link failed: Bad value",
											"tests": "dh_auto_test: .* returned exit code 2",
}

//Update the cause field for FTBFS records based on a patterns matching their error logs
func updateCauses() {
	for cause, p := range errorPatterns {
		collection.UpdateAll(bson.M{"content":bson.RegEx{Pattern:p}}, bson.M{"$set": bson.M{"cause":cause}})
	}
}

func save(b lpad.Build, spph lpad.SPPH) {

	url := b.BuildLogURL()

	if stored(url) {
		return
	}

	content := getBuildLog(url)

	fmt.Printf("Saving error log for %s %s %s\n", spph.PackageName(), spph.PackageVersion(), b.ArchTag())

	collection.Insert(bson.M{"url":url, "cause": "other", "content":content, "datecreated":b.DateCreated()})
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
	var fetch, update bool

	flag.BoolVar(&fetch, "f", false, "Fetch recent FTBFS data from Launhpad and store it to the database")
	flag.BoolVar(&update, "u", false, "Update the FTBFS cause field on saved build records")

	flag.Parse()

	mongoConnect()
	if fetch {
		root := login()
		getFTBFS(root, "")
	}

	if update {
		updateCauses()
	}
}
