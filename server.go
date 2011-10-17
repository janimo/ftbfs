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
}

type P struct {
	Entries []Entry
}

var p P

//We're optimists and hope there won't be over 1000 FTBFS records :)

func fillEntries() {
	var entry *Entry
	p = P{}
	p.Entries = make([]Entry, 1000)
	q := collection.Find(nil)
	i := 0
	q.For(&entry, func() os.Error {
		p.Entries[i] = *entry
		i++
		return nil
	})
	p.Entries = p.Entries[:i]
}

func viewHandle(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFile("templates/view.html")
	if err != nil {
		http.Error(w, err.String(), http.StatusInternalServerError)
	}
	t.Execute(w, p)
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

	http.HandleFunc("/view/", viewHandle)

	pwd, _ := os.Getwd()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(pwd+"/static/"))))

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		println(err.String())
	}

	s<-1
}
