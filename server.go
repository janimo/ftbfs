package main

import (
	"os"
	"time"
	"http"
	"template"
)

//DB fields to be used in the template
type Entry struct {
	Package     string
	Version     string
	Cause       string
	Datecreated string
	Component   string
	Content     string
	URL         string
}

type P struct {
	Entries map[string]Entry
}

var p P

func fillEntries() {
	var entry *Entry
	p = P{}
	p.Entries = make(map[string]Entry)
	q := collection.Find(nil)

	q.For(&entry, func() os.Error {
		p.Entries[entry.Package] = *entry
		return nil
	})
}

func viewHandle(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFile("templates/view.html")
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
	}
	t.Execute(w, p)
}

func logViewHandle(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFile("templates/logview.html")
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
	}
	pkg := r.URL.Path[9:]
	t.Execute(w, p.Entries[pkg])
}

func updateEntries() {
	c := time.Tick(3e9)
	for {
		fillEntries()
		<-c
	}
}

//Start the web server
func runServer(port string, s chan int) {
	go updateEntries()

	http.HandleFunc("/", viewHandle)
	http.HandleFunc("/logview/", logViewHandle)

	pwd, _ := os.Getwd()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(pwd+"/static/"))))

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		println(err.String())
	}

	s <- 1
}
