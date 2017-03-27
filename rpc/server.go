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

/*
   TServer:维护一个rpcserver服务
   1）监听端口，接收连接，每接到一个连接创建一个TServerConn负责相应服务工作
   2）接受注册服务，可将一个变量所有符合要求的方法注册到服务中
   3）维护TRequestHeader TResponseHeader对象池（列表）
   4）根据请求方法名，返回对应的TService TMethodType



  注册方法的条件：
  - 导出方法（首字母大写）
  - 2个参数，且都是导出类型
  - 第二个参数是指针
  - 1个返回值，类型为error
*/

package rpc

import (
	log "babeltime.com/log4go"
	"container/list"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	RPC_SERVER_STATUS_NEW     = iota //新建对象，还未调用Start开始服务
	RPC_SERVER_STATUS_RUNNING        //调用完Start，开始服务
	RPC_SERVER_STATUS_STOPING        //调用Stop，停止服务，等所有执行中的请求执行完毕后，停止服务
	RPC_SERVER_STATUS_STOPED
)

type TServer struct {
	mu               sync.RWMutex //在运行需要频繁读serviceMap，使用读写锁
	addr             string
	listener         net.Listener
	codecFactory     FServerCodecFactory //使用编解码器生成函数，方便扩展不同的编解码方式
	dispatchFunc     FDispatchCall       //外部设置的分发请求的函数
	serviceMap       map[string]*TService
	logger           *log.Logger
	conf             *TServerConfig
	serverConnWg     sync.WaitGroup
	serverConnListMu sync.Mutex
	serverConnList   *list.List
	status           int32
}

func NewServer(aAddr string, aCodecFactory FServerCodecFactory, aConfig *TServerConfig) (server *TServer, err error) {
	logger := log.NewLogger()

	// tcpAddr, err := net.ResolveTCPAddr("tcp4", aAddr)
	// if err != nil {
	//     logger.Fatal("NewServer error:" + err.Error())
	//     return
	// }
	// listener, err := net.ListenTCP("tcp4", tcpAddr)
	// if err != nil {
	//     logger.Fatal("NewServer error:" + err.Error())
	//     return
	// }
	netType, addr, err := getNetAddr(aAddr, true)
	if err != nil {
		logger.Warning("NewServer getNetAddr addr:%s err:%s", addr, err)
		return
	}
	logger.Debug("NewServer netType:%s addr:%s", netType, addr)
	listener, err := net.Listen(netType, addr)
	if err != nil {
		logger.Fatal("NewServer error:" + err.Error())
		return
	}

	server = &TServer{
		addr:           aAddr,
		listener:       listener,
		codecFactory:   aCodecFactory,
		serviceMap:     make(map[string]*TService),
		logger:         logger,
		conf:           aConfig,
		serverConnList: list.New(),
		status:         RPC_SERVER_STATUS_NEW,
	}
	return
}

func (this *TServer) Register(aRcvr interface{}) error {
	return this.register(aRcvr, "", false)
}

func (this *TServer) RegisterName(aName string, aRcvr interface{}) error {
	return this.register(aRcvr, aName, true)
}

func (this *TServer) register(aRcvr interface{}, aName string, aUseName bool) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.serviceMap == nil {
		this.serviceMap = make(map[string]*TService)
	}
	s := new(TService)
	s.typ = reflect.TypeOf(aRcvr)
	s.rcvr = reflect.ValueOf(aRcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if aUseName {
		sname = aName
	}
	if sname == "" {
		errMsg := "rpc.Register: no service name for type " + s.typ.String()
		this.logger.Fatal("%s", errMsg)
		return errors.New(errMsg)
	}
	if !IsExported(sname) && !aUseName {
		errMsg := "rpc.Register: type " + sname + " is not exported"
		this.logger.Fatal("%s", errMsg)
		return errors.New(errMsg)
	}
	if _, present := this.serviceMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname

	// Install the methods
	s.method = this.suitableMethods(s.typ, sname, true)

	if len(s.method) == 0 {
		errMsg := ""

		// To help the user, see if a pointer receiver would work.
		method := this.suitableMethods(reflect.PtrTo(s.typ), sname, false)
		if len(method) != 0 {
			errMsg = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			errMsg = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		this.logger.Fatal("%s", errMsg)
		return errors.New(errMsg)
	}
	this.logger.Info("register done. servieName:%s, rcvrType:%s methodNum:%d", sname, s.typ, len(s.method))
	this.serviceMap[s.name] = s
	return nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if aReportErr is true.
func (this *TServer) suitableMethods(aRcvrType reflect.Type, aServiceName string, aReportErr bool) map[string]*TMethodType {

	methods := make(map[string]*TMethodType)
	for m := 0; m < aRcvrType.NumMethod(); m++ {
		method := aRcvrType.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 3 {
			if aReportErr {
				this.logger.Info("servieName:%s, method:%s has wrong number of ins:%d", aServiceName, mname, mtype.NumIn())
			}
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(1)
		if !IsExportedOrBuiltinType(argType) {
			if aReportErr {
				this.logger.Warning("servieName:%s, method:%s argument type not exported:%v", aServiceName, mname, argType)
			}
			continue
		}
		// Second arg must be a pointer.
		replyType := mtype.In(2)
		if replyType.Kind() != reflect.Ptr {
			if aReportErr {
				this.logger.Warning("servieName:%s, method:%s reply type not a pointer:%v", aServiceName, mname, replyType)
			}
			continue
		}
		// Reply type must be exported.
		if !IsExportedOrBuiltinType(replyType) {
			if aReportErr {
				this.logger.Warning("servieName:%s, method:%s reply type not exported:%v", aServiceName, mname, replyType)
			}
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			if aReportErr {
				this.logger.Warning("servieName:%s, method:%s has wrong number of outs:%v", aServiceName, mname, replyType)
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			if aReportErr {
				this.logger.Warning("servieName:%s, method:%s returns:%s not error", aServiceName, mname, returnType.String())
			}
			continue
		}
		this.logger.Info("found method. servieName:%s, rcvrType:%s methodName:%s", aServiceName, aRcvrType, mname)
		methods[mname] = &TMethodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

func (this *TServer) Start() {

	this.logger.Info("server started. listen on:%s", this.listener.Addr().String())

	atomic.CompareAndSwapInt32(&this.status, RPC_SERVER_STATUS_NEW, RPC_SERVER_STATUS_RUNNING)
	for {
		conn, err := this.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				this.logger.Fatal("stop accepting connections")
				return
			}
			this.logger.Warning("accept failed:%s", err.Error())
			time.Sleep(time.Second)
			continue
		}

		this.logger.Info("client:%s connected", conn.RemoteAddr().String())
		serverConn := NewServerConn(this, conn, this.conf.MaxWaitResp)

		this.serverConnListMu.Lock()
		this.serverConnList.PushBack(serverConn)
		this.serverConnListMu.Unlock()

		serverConn.Start()
	}
}

func (this *TServer) Stop() {
	oldStatus := this.status
	if atomic.CompareAndSwapInt32(&this.status, RPC_SERVER_STATUS_RUNNING, RPC_SERVER_STATUS_STOPING) == false {
		this.logger.Fatal("stop failed. status:%d,%d", oldStatus, this.status)
		return
	}

	//让Accept立刻返回超时错误，停止接收连接的routine
	//this.listener.SetDeadline(time.Now())
	err := this.listener.Close()
	if err != nil {
		this.logger.Warning("Stop this.listen.Close() err:%s", err)
	}

	this.serverConnListMu.Lock()
	for e := this.serverConnList.Front(); e != nil; e = e.Next() {
		e.Value.(*TServerConn).Stop()
	}
	this.serverConnListMu.Unlock()
}

func (this *TServer) SetDispatchFunc(aFunc FDispatchCall) {
	this.dispatchFunc = aFunc
}

func (this *TServer) dispatchRequest(aCall *TFuncCall) (bool, error) {
	if this.dispatchFunc == nil {
		return false, nil
	}
	return this.dispatchFunc(aCall)
}

func (this *TServer) getSeviceMethod(aServiceMethod string) (service *TService, mtype *TMethodType, err error) {
	dot := strings.Index(aServiceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc: service/method request ill-formed: " + aServiceMethod)
		return
	}
	serviceName := aServiceMethod[:dot]
	methodName := aServiceMethod[dot+1:]

	//考虑到在运行中可能增加新的service，但是一个service下不会增加新的method，所以前面需要加锁，后面不需要
	this.mu.RLock()
	service = this.serviceMap[serviceName]
	this.mu.RUnlock()
	if service == nil {
		err = errors.New("rpc: can't find service " + aServiceMethod)
		return
	}
	mtype = service.method[methodName]
	if mtype == nil {
		err = errors.New("rpc: can't find method " + aServiceMethod)
		return
	}
	return
}

func (this *TServer) OneServerConnStart() {
	this.serverConnWg.Add(1)
}

func (this *TServer) OneServerConnStop(aServerConn *TServerConn) {
	this.serverConnWg.Done()

	this.serverConnListMu.Lock()
	defer this.serverConnListMu.Unlock()
	for e := this.serverConnList.Front(); e != nil; e = e.Next() {
		if e.Value == aServerConn {
			this.logger.Info("remove one connection")
			this.serverConnList.Remove(e)
			break
		}
	}
}

func (this *TServer) isRunning() bool {
	return this.status == RPC_SERVER_STATUS_RUNNING
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
