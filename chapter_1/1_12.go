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
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
	http.HandleFunc("/count", counter)
}

// handler echoes the Path component of the request URL r.
func handler(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "URL.Path = %q\n", r.URL.Path)
	mu.Lock()

	count++

	mu.Unlock()
}

func counter(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "URL.Path====%q\n", r.URL.Path)
	mu.Lock()

	fmt.Fprintf("counter is %d", count)

	mu.Unlock()
}
