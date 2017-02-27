package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	// 1_1
	fmt.Println(strings.Join(os.Args[0:], ""))

	// 1_2
	for k, v := range os.Args {
		fmt.Printf("index %d content %s\n", k, v)
	}
	//1_3
}
