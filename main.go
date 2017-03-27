package main

import (
	// "bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	// "log"
	// "io"
	"io/ioutil"
	"os"
)

// type TTactic struct {
	// TacticInfo map[int32][]uint32
// }

func main() {
	gob.Register(make(map[string][]uint32))
	gob.Register(make([]interface{}, 0))
	f, err := os.Open("./tactic")
	if err != nil {
		panic(err)
	}
	var ret map[int32][]uint32	
	// reader := bufio.NewReader(f)
	// fmt.Println("io SeekEnd",io.SeekEnd)
	bts, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println("...", err)
		panic(err)
	}
	// fmt.Println("...", len(bts))
	// temp :=bts[0:len(bts)-2]
	// fmt.Println("...",len(temp))
	buffer := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buffer)
	err = dec.Decode(&ret)
	if err != nil {
		// fmt.Println("...", err)
		panic(err)
	}
	fmt.Println("t%#v", ret)
}
