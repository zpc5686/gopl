/**********************************************************************************************************************
 *
 * Copyright (c) 2010 babeltime.com, Inc. All Rights Reserved
 * $Id$
 *
 **********************************************************************************************************************/

/**
 * @file $HeadURL$
 * @author $Author$(wuqilin@babeltime.com)
 * @date $Date$
 * @version $Revision$
 * @brief
 *
 **/

package main

import (
	log "babeltime.com/log4go"
	file "babeltime.com/log4go/file"
	"babeltime.com/rpc"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var FILE_PREFIX string

func runClient(aIndex int, aChildNum int, aClient *rpc.TClient, aRunTime time.Duration, aCh chan<- int) {

	logger := log.NewLogger()
	logger.AddInfo("rpc_client_test_index", fmt.Sprintf("rpc_client_test_%d", aIndex))

	var retMsg string
	msgBuf := make([]byte, 100)
	for j := 0; j < len(msgBuf); j++ {
		msgBuf[j] = byte('0' + (j % 80))
	}
	sendMsg := string(msgBuf)

	startTime := time.Now()
	var timeDelt time.Duration
	var lastPrint time.Duration
	printDelt := time.Second * time.Duration(2*aChildNum)
	if printDelt > time.Second*5 {
		printDelt = time.Second * 5
	}
	numRun := 0
	numSuc := 0
	for {
		st := time.Now()
		err := aClient.SyncCall("test.Echo", sendMsg, &retMsg)

		numRun++
		if err == nil {
			numSuc++
		} else {
			logger.Fatal("get err:%v", err)
		}

		logger.Notice("done one cost:%f", time.Since(st).Seconds())

		timeDelt = time.Since(startTime)

		if timeDelt-lastPrint > printDelt {
			lastPrint = timeDelt
			fmt.Printf("[%d]send msg. run:%d, suc:%d, now:%v, runTime:%v\n", aIndex, numRun, numSuc, timeDelt, aRunTime)
		}
		if timeDelt >= aRunTime {
			logger.Notice("runTime:%v done", aRunTime)
			break
		}
	}

	aCh <- numRun
	aCh <- numSuc
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	var pAddr = flag.String("addr", "127.0.0.1:1235", "")
	var pLogLevel = flag.Int("loglevel", 3, "")
	var pChildNum = flag.Int("child", 1, "")
	var pConnNum = flag.Int("conn", 1, "")
	var pRunTime = flag.Int("time", 0, "")

	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memProfile = flag.String("memprofile", "", "write mem profile to file")
	var blockProfile = flag.String("blockprofile", "", "write mem profile to file")
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can not create cpu profile output file: %s",
				err)
			return
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Can not start cpu profile: %s", err)
			f.Close()
			return
		}
		defer pprof.StopCPUProfile()
		println("cpu profile " + *cpuProfile)
	}
	if *memProfile != "" {
		println("mem profile " + *memProfile)
		defer func() {
			f, err := os.Create(*memProfile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can not create mem profile output file: %s", err)
				return
			}
			if err = pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintf(os.Stderr, "Can not write %s: %s", *memProfile, err)
			}
			f.Close()
		}()
	}
	if *blockProfile != "" {
		println("block profile " + *memProfile)
		defer func() {
			f, err := os.Create(*blockProfile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can not create block profile output file: %s", err)
				return
			}
			if err = pprof.Lookup("block").WriteTo(f, 0); err != nil {
				fmt.Fprintf(os.Stderr, "Can not write %s: %s", *blockProfile, err)
			}
			f.Close()
		}()

	}

	addr := *pAddr
	logLevel := log.LogLevel(*pLogLevel)
	childNum := *pChildNum
	connNum := *pConnNum
	runTime := time.Second * time.Duration(*pRunTime)

	//	log.InitLog(logLevel, "log/client", FILE_PREFIX)
	file.SetFile("log/client")
	log.SetLevel(logLevel)
	logger := log.NewLogger()

	logger.Notice("server add:%s, childNum:%d, connNum:%d, runTime:%v", addr, childNum, connNum, runTime)

	config := rpc.TClientConfig{
		MaxWaitCall:    100,
		MaxRetryNum:    1,
		PingInterval:   time.Second * 10,
		RequestTimeout: time.Second * 5,
	}
	client := rpc.NewClient("test", &config, rpc.NewClientCodec)

	client.AddServer(addr, 1, connNum)

	var startTime = time.Now()

	var arrCh = make([]chan int, childNum)
	for i := 0; i < childNum; i++ {
		arrCh[i] = make(chan int)
		go runClient(i, childNum, client, runTime, arrCh[i])
	}

	var sumRun = 0
	var sumSuc = 0
	for _, ch := range arrCh {
		numRun := <-ch
		numSuc := <-ch
		sumRun += numRun
		sumSuc += numSuc
	}

	if sumSuc < sumRun {
		logger.Fatal("sumSuc:%d < sumRun:%d", sumSuc, sumRun)
	}

	var endTime = time.Now()
	var delt = endTime.Sub(startTime)
	msg := fmt.Sprintf("sumRun:%d sumSuc:%d delt:%f avg:%f\n", sumRun, sumSuc, delt.Seconds(), float64(sumSuc)/delt.Seconds())
	fmt.Println(msg)
	logger.Notice("%s", msg)

	time.Sleep(time.Millisecond * 500)
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
