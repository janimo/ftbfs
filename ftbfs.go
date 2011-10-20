package main

import (
	"os"
	"fmt"
	"http"
	"flag"
	"time"
	"io/ioutil"
	"strings"
	"launchpad.net/gobson/bson"
	"launchpad.net/mgo"
	"launchpad.net/~jani/lpad/changes"
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

//Line in the buildlogs immediately following the error messages
const endOfLog = "FAILED [dpkg-buildpackage died]"

//Build error messages should only be looked for in the lastBytes of the log text
const lastBytes = 100 * 1024

//Retrieve the gist of the error logs pointed to by the URL
func getBuildLog(url string) string {
	response, err := http.Get(url)
	check(err)
	defer response.Body.Close()

	b, err := ioutil.ReadAll(response.Body)
	check(err)

	//only the last part of the log is relevant
	e := len(b) - 1
	s := 0
	if e > lastBytes {
		s = e - lastBytes
	}

	//drop anything after the error message
	res := strings.SplitN(string(b[s:e]), endOfLog, 2)

	//keep only last 200 lines
	split := strings.Split(res[0], "\n")

	e = len(split) - 1
	s = 0
	if e > 200 {
		s = e - 200
	}

	return strings.Join(split[s:e], "\n")
}

//Has a value for all architectures we care about, currently only ARM
var archs = map[string]bool{"armel": true}

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
	q := collection.Find(bson.M{"url": url})
	c, err := q.Count()
	check(err)
	return c > 0
}

//A set of hardcoded matches. Needs to allow user specified patterns to be more useful and generic
var errorPatterns = map[string]string{
	"timeout":      "Build killed with signal 15",
	"segfault":     "Segmentation fault",
	"gcc-ice":      "internal compiler error",
	"cmake":        "CMake Error",
	"opengl":       "error: '(GL|gl).* was not declared",
	"qtopengl":     "error: 'GLdouble' has a previous declaration",
	"linker":       "final link failed: Bad value",
	"tests":        "dh_auto_test: .* returned exit code 2",
	"dependencies": "but it is not going to be installed",
	"warnings":     "some warnings being treated as errors",
}

type PatternCause struct {
	Pattern string //string occuring in the error log
	Cause   string //name of the corresponding error cause
}

//Update the cause field for FTBFS records based on a patterns matching their error logs
func updateCauses() {

	var c *PatternCause

	q := causes.Find(nil)

	q.For(&c, func() os.Error {
		errorPatterns[c.Cause] = c.Pattern
		return nil
	})

	for cause, p := range errorPatterns {
		collection.UpdateAll(bson.M{"content": bson.RegEx{Pattern: p}}, bson.M{"$set": bson.M{"cause": cause}})
	}
}

var ftbfsList map[string]bool

func save(b lpad.Build, spph lpad.SPPH) {

	url := b.BuildLogURL()

	ftbfsList[url] = false, false

	if stored(url) {
		return
	}

	content := getBuildLog(url)

	fmt.Printf("Saving error log for %s %s %s\n", spph.PackageName(), spph.PackageVersion(), b.ArchTag())

	collection.Upsert(bson.M{"package": spph.PackageName()}, bson.M{"package": spph.PackageName(), "version": spph.PackageVersion(), "url": url, "cause": "other", "content": content, "datecreated": b.DateCreated(), "component": spph.Component()})
}

type FTBFSEntry struct {
	URL string
}

func loadFTBFSList() {
	var entry *FTBFSEntry

	if ftbfsList == nil {
		ftbfsList = make(map[string]bool)
	}
	q := collection.Find(nil)

	q.For(&entry, func() os.Error {
		ftbfsList[entry.URL] = true
		return nil
	})

}

//Remove old FTBFS entries
func purgeFTBFSList() {
	for url,_ := range(ftbfsList) {
		collection.Remove(bson.M{"url":url})
	}
}

//Find current FTBFS logs
func getFTBFS(root lpad.Root) {
	ubuntu, _ := root.Distro("ubuntu")
	series, _ := ubuntu.FocusSeries()
	fmt.Println("Fetching FTBFS list for", series.FullSeriesName())

	loadFTBFSList()

	for _, build_state := range []lpad.BuildState{lpad.BSFailedToBuild} {
		ftbfs, _ := series.GetBuildRecords(build_state, lpad.PocketRelease, "")
		ftbfs.For(func(b lpad.Build) os.Error {
			process(b, build_state)
			return nil
		})
	}

	purgeFTBFSList()
	fmt.Println("Done fetching")
}

//nanoseconds in hour
const HOUR = 1e9*3600

func taskGetFTBFS(root lpad.Root) {
	c := time.Tick(1 * HOUR)
	for {
		getFTBFS(root)
		fillEntries()
		<-c
	}
}

//Struct into which Query.For() unmarshals
var result *struct{ URL string }

func queryFTBFS(cause string) {
	q := collection.Find(bson.M{"cause": cause})
	c, _ := q.Count()
	fmt.Printf("A total of %d packages FTBFS with cause '%s'\n\n", c, cause)

	q.For(&result, func() os.Error {
		fmt.Println(result.URL)
		return nil
	})
}

const (
	MONGO_URL        = "localhost"
	MONGO_DB         = "FTBFS"
	MONGO_COL_FTBFS  = "ftbfs"
	MONGO_COL_CAUSES = "causes"
)

var collection, causes mgo.Collection

func mongoConnect() {
	session, err := mgo.Mongo(MONGO_URL)
	check(err)

	collection = session.DB(MONGO_DB).C(MONGO_COL_FTBFS)
	causes = session.DB(MONGO_DB).C(MONGO_COL_CAUSES)
}

func main() {
	var fetch, update, serve bool
	var cause, port string

	flag.BoolVar(&fetch, "f", false, "Fetch recent FTBFS data from Launhpad and store it to the database")
	flag.BoolVar(&update, "u", false, "Update the FTBFS cause field on saved build records")
	flag.BoolVar(&serve, "s", false, "Start the web server")
	flag.StringVar(&port, "p", "9999", "Port to listen on")
	flag.StringVar(&cause, "c", "", "List FTBFS for a given cause (i.e. timeout, opengl)")

	flag.Parse()

	mongoConnect()
	root := login()

	s := make(chan int)

	if serve {
		fillEntries()
		go runServer(port, s)
	}

	if fetch {
		if serve {
			go taskGetFTBFS(root)
		} else {
			getFTBFS(root)
		}
	}

	if update {
		updateCauses()
	}

	if cause != "" {
		queryFTBFS(cause)
	}

	if flag.NFlag() == 0 {
		fmt.Println("Usage:")
		flag.PrintDefaults()
	}

	if serve {
		<-s
	}
}
