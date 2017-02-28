package main

import (
	"fmt"
	"io"
	// "io/ioutil"
	"net/http"
	"os"
	"strings"
)

func main() {
	for _, url := range os.Args[1:] {

		if !strings.HasPrefix(url, "http://") {
			url = "http://" + url
		}
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("err %v", err)
			os.Exit(1)
		}

		_, err = io.Copy(os.Stdout, resp.Body)
		resp.Body.Close()

		if err != nil {
			fmt.Printf("io copy err %v", err)
		}
		fmt.Println("statue code", resp.StatusCode)

		// body, _ := ioutil.ReadAll(resp.Body)
		// fmt.Printf("%s", body)
	}
}
