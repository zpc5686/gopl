package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	// counts := make(map[string]int)
	// input := bufio.NewScanner(os.Stdin)

	// for input.Scan() {
	// 	counts[input.Text()]++
	// }

	// for k, v := range counts {
	// 	fmt.Printf("%d \t %s \n", v, k)
	// }

	counts := make(map[string]int)

	files := os.Args[1:]

	for _, v := range files {
		f, err := os.Open(v)
		if err != nil {
			fmt.Println("open file err %v", err)
			return
		}

		countLines(counts, f)
		f.Close()
	}
	for line, n := range counts {
		if n > 1 {
			fmt.Printf("%d\t%s\n", n, line)
		}
	}
}

func countLines(counts map[string]int, f *os.File) {
	input := bufio.NewScanner(f)
	for input.Scan() {
		counts[input.Text()]++
		if counts[input.Text()] > 1 {
			fmt.Println(f.Name())
		}
	}

}
