package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

var mu sync.Mutex

var count int

func main() {
	http.HandleFunc("/", handler) // each request calls handler

	http.HandleFunc("/count", counter)
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

// handler echoes the Path component of the request URL r.
func handler(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "URL.Path = %q\n", r)
	if err := r.ParseForm(); err != nil {
		log.Print(err)
	}
	for k, v := range r.Form {
		fmt.Fprintf(w, "Form[%q] = %q\n", k, v)
	}
}

func counter(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "URL.Path====%q\n", r.URL.Path)
	mu.Lock()

	fmt.Fprintf(w, "Count %d\n", count)

	mu.Unlock()
}
