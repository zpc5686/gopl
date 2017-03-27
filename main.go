package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	// "log"
	"io"
	// "io/ioutil"
	"os"
)

type TTactic struct {
	TacticInfo map[int32][]uint32
}

func main() {
	gob.Register(make(map[string]interface{}))
	gob.Register(make([]interface{}, 0))
	f, err := os.Open("./tactic")
	if err != nil {
		panic(err)
	}
	var ret TTactic
	reader := bufio.NewReader(f)
	bts, err := reader.ReadBytes(io.SeekEnd)
	if err != nil {
		fmt.Println("readbytes", err)
		panic(err)
	}
	fmt.Println("...", bts)
	buffer := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buffer)
	err = dec.Decode(&ret)
	if err != nil {
		fmt.Println("...", err)
		panic(err)
	}
	fmt.Println("t%#v", ret.TacticInfo)
}
