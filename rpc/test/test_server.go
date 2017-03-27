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

//import (
//	log "babeltime.com/log4go"
//	"babeltime.com/rpc"
//	"flag"
//	"fmt"
//	"os"
//	"os/signal"
//	"runtime/pprof"
//	"sync/atomic"
//	"time"
//)
//
//var FILE_PREFIX string
//
//type TTestService struct {
//	counter int64
//}
//
//func (this *TTestService) Echo(aMsg string, retMsg *string) error {
//	*retMsg = fmt.Sprintf("[%d]%s", atomic.LoadInt64(&this.counter), aMsg)
//	atomic.AddInt64(&this.counter, 1)
//	return nil
//}
//
//func waitForSignal() os.Signal {
//	signalChan := make(chan os.Signal, 1)
//	defer close(signalChan)
//
//	signal.Notify(signalChan, os.Kill, os.Interrupt)
//	s := <-signalChan
//	signal.Stop(signalChan)
//	return s
//}
//
//func main() {
//
//	var pAddr = flag.String("addr", ":1235", "")
//	var pLogLevel = flag.Int("loglevel", 3, "")
//	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
//	var memProfile = flag.String("memprofile", "", "write mem profile to file")
//	var blockProfile = flag.String("blockprofile", "", "write mem profile to file")
//	flag.Parse()
//
//	if *cpuProfile != "" {
//		f, err := os.Create(*cpuProfile)
//		if err != nil {
//			fmt.Fprintf(os.Stderr, "Can not create cpu profile output file: %s",
//				err)
//			return
//		}
//		if err := pprof.StartCPUProfile(f); err != nil {
//			fmt.Fprintf(os.Stderr, "Can not start cpu profile: %s", err)
//			f.Close()
//			return
//		}
//		defer pprof.StopCPUProfile()
//		println("cpu profile " + *cpuProfile)
//	}
//	if *memProfile != "" {
//		println("mem profile " + *memProfile)
//		defer func() {
//			f, err := os.Create(*memProfile)
//			if err != nil {
//				fmt.Fprintf(os.Stderr, "Can not create mem profile output file: %s", err)
//				return
//			}
//			if err = pprof.WriteHeapProfile(f); err != nil {
//				fmt.Fprintf(os.Stderr, "Can not write %s: %s", *memProfile, err)
//			}
//			f.Close()
//		}()
//	}
//	if *blockProfile != "" {
//		println("block profile " + *memProfile)
//		defer func() {
//			f, err := os.Create(*blockProfile)
//			if err != nil {
//				fmt.Fprintf(os.Stderr, "Can not create block profile output file: %s", err)
//				return
//			}
//			if err = pprof.Lookup("block").WriteTo(f, 0); err != nil {
//				fmt.Fprintf(os.Stderr, "Can not write %s: %s", *blockProfile, err)
//			}
//			f.Close()
//		}()
//
//	}
//
//	addr := *pAddr
//	logLevel := log.LogLevel(*pLogLevel)
//
//	log.InitLog(logLevel, "log/rpc_server", FILE_PREFIX)
//
//	logger := log.NewLogger()
//
//	logger.Info("addr:%s", addr)
//
//	serverConf := &rpc.TServerConfig{
//		ServerIdleTimeout: time.Second * 60,
//	}
//	server, err := rpc.NewServer(addr, rpc.NewServerCodec, serverConf)
//
//	if err != nil {
//		logger.Fatal("err: %v", err)
//	}
//
//	err = server.RegisterName("test", &TTestService{})
//	if err != nil {
//		logger.Fatal("regist failed err: %v", err)
//		return
//	}
//	go server.Start()
//
//	s := waitForSignal()
//
//	logger.Info("get signal:%v", s)
//
//	//time.Sleep(100 * time.Millisecond)
//}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
