package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	runtime.GOMAXPROCS(1)
	var workResultLock sync.WaitGroup
	workResultLock.Add(1)
	go func() {
		defer workResultLock.Done()
		fmt.Println("我开始跑了哦。。。")
		i := 1
		for {

			i++
			break
		}
		runtime.Gosched()
	}()
	workResultLock.Add(1)
	time.Sleep(time.Second * 2)
	go func() {
		defer workResultLock.Done()
		fmt.Println("我还有机会吗？？？？")
	}()

	workResultLock.Wait()

	fmt.Println("单线程")
}
