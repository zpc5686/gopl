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
   TServerConn：维护一个外部进来的连接
   完整的主要工作
    1）循环接受请求
    2）收到请求后启动一个routine完整对应的工作
    3）返回响应

    TODO
    1）处理ping请求
*/

package rpc

import (
	log "babeltime.com/log4go"
	"errors"
	"io"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"time"
)

const (
	UNITE_PACKAGE_NUM = 100
)

var gFuncCallPool *sync.Pool

func init() {
	gFuncCallPool = &sync.Pool{
		New: func() interface{} {
			return new(TFuncCall)
		},
	}
}

func NewFuncCall() *TFuncCall {
	return gFuncCallPool.Get().(*TFuncCall)
}

func FreeFuncCall(aFuncCall *TFuncCall) {
	*aFuncCall = TFuncCall{}
	gFuncCallPool.Put(aFuncCall)
}

func WhenFuncCallDone(aFuncCall *TFuncCall) {
	//需要在做完活后通知写响应的请求，会在写完响应后Free；其他的（dummyReturn）的直接
	if aFuncCall.needSendRespWhenDone() {
		aFuncCall.Done <- aFuncCall
	} else {
		FreeFuncCall(aFuncCall)
	}
}

type TServerConn struct {
	conn         net.Conn
	codec        IServerCodec
	server       *TServer
	logger       *log.Logger
	doneChan     chan *TFuncCall
	sendingMutex sync.Mutex //串化SendResponse
	pendingWg    sync.WaitGroup
}

type TServerGroupResponse struct {
	resp TResponseHeader
	call *TFuncCall
}

func NewServerConn(aServer *TServer, aConn net.Conn, aMaxWaitResp uint32) *TServerConn {

	logger := log.NewLogger()

	codec := aServer.codecFactory(aConn)
	return &TServerConn{
		conn:     aConn,
		server:   aServer,
		codec:    codec,
		logger:   logger,
		doneChan: make(chan *TFuncCall, aMaxWaitResp),
	}
}

func (this *TServerConn) Start() {
	this.logger.Debug("TServerConn Start")
	go this.RoutineRun()
	go this.RoutineWriteResponse()
}

func (this *TServerConn) Stop() {
	this.conn.SetReadDeadline(time.Now())
}

func (this *TServerConn) RoutineRun() {
	defer func() {
		if r := recover(); r != nil {
			this.logger.Fatal("client:%d erro, recover:%v, stack:\n%s", this.conn.RemoteAddr().String(), r, debug.Stack())
			close(this.doneChan)
			this.codec.Close()
		}
	}()

	this.logger.Info("start server")
	this.server.OneServerConnStart()
	defer this.server.OneServerConnStop(this)

	for {

		if err := this.conn.SetReadDeadline(time.Now().Add(this.server.conf.ServerIdleTimeout)); err != nil {
			this.logger.Fatal("SetReadDeadline failed. err:%s", err.Error())
			break
		}

		service, mtype, req, argv, replyv, keepReading, err := this.ReadRequest()

		if !this.server.isRunning() {
			//只有在不同主动停止时，才会等待所有请求执行完毕
			this.logger.Warning("server stoped, wait all pending request done")
			this.pendingWg.Wait()
			this.logger.Warning("all pending request done")
			break
		}

		this.pendingWg.Add(1)

		if err != nil {
			if err != io.EOF {
				this.logger.Warning("readRequest failed:%s", err.Error())
			}
			if !keepReading {
				this.logger.Warning("readRequest failed stop service. err:%s ", err.Error())
				break
			}
			// send a response if we actually managed to read a header.
			if req.ServiceMethod != "" {
				this.logger.Warning("readRequest failed. but get the header, send err resposne. continue service")
				this.SendResponse(req, nil, err.Error())
			}
			continue
		}
		this.logger.Trace("recv one request:%s %v", req.ServiceMethod, req)

		if req.IsPing() {
			this.SendResponse(req, nil, "")
			continue
		}

		call := NewFuncCall()
		call.MethodName = req.ServiceMethod
		call.Func = mtype.method.Func
		call.Rcvr = service.rcvr
		call.Arg = argv
		call.Reply = replyv
		call.Header = req
		call.Done = nil

		if req.DummyReturn() {
			//对方不需要执行结果，我们可以先返回，然后再执行函数
			this.logger.Debug("method:%s dummy return", req.ServiceMethod)
			this.doWriteResponse(call)
		} else {
			call.Done = this.doneChan
		}

		hasDisp, err := this.server.dispatchRequest(call)
		if err != nil {
			this.SendResponse(req, nil, err.Error())
			continue
		}

		if !hasDisp {
			//如果请求没有被分发，就起一个临时routine干活
			this.logger.Debug("req:%s not dispatched", req.ServiceMethod)
			go this.RoutineDoCall(call)
		}
	}

	this.logger.Info("client done")
	close(this.doneChan)
	this.codec.Close()

}

func (this *TServerConn) RoutineWriteResponse() {

	defer func() {
		if r := recover(); r != nil {
			this.logger.Fatal("client:%d erro, recover:%v, stack:\n%s", this.conn.RemoteAddr().String(), r, debug.Stack())
		}
	}()

	this.logger.Info("start RoutineWriteResponse")

	running := true
	for running {
		select {
		case call, ok := <-this.doneChan:
			if !ok {
				this.logger.Warning("doneChan closed")
				running = false
				break
			}
			if !this.doWriteResponse(call) {
				this.logger.Warning("doneChan closed")
				running = false
				break
			}
		}
	}

	this.logger.Info("RoutineWriteResponse done")
}

func (this *TServerConn) doCallResponse(aCall *TFuncCall) TResponseHeader {
	errmsg := ""
	if len(aCall.Ret) > 0 {
		errInter := aCall.Ret[0].Interface()
		if errInter != nil {
			errmsg = errInter.(error).Error()
		}
	}
	this.logger.Debug("write response for req:%s %v", aCall.MethodName, aCall)
	respHeader := this.makeRespHeader(aCall.Header, aCall.Reply.Interface(), errmsg)
	return respHeader
}

func (this *TServerConn) doWriteResponse(aCall *TFuncCall) bool {
	respHeader := this.doCallResponse(aCall)
	var groupResponse []TServerGroupResponse
	resp := TServerGroupResponse{
		resp: respHeader,
		call: aCall,
	}
	groupResponse = append(groupResponse, resp)
	looping := true
	for i := 0; i < UNITE_PACKAGE_NUM && looping; i++ {
		select {
		case call, ok := <-this.doneChan:
			if !ok {
				return false
			}
			respHeader = this.doCallResponse(call)
			resp := TServerGroupResponse{
				resp: respHeader,
				call: call,
			}
			groupResponse = append(groupResponse, resp)
		default:
			looping = false
		}
	}

	this.SendGroupResponse(groupResponse)
	return true
}

func (this *TServerConn) RoutineDoCall(aCall *TFuncCall) {
	defer func() {
		if r := recover(); r != nil {
			this.logger.Fatal("client:%d RoutineDoCall err, recover:%v, stack:\n%s", this.conn.RemoteAddr().String(), r, debug.Stack())
		}
	}()

	this.logger.Debug("RoutineDoCall %v", aCall.Arg)
	aCall.Ret = aCall.Func.Call([]reflect.Value{aCall.Rcvr, aCall.Arg, aCall.Reply})

	WhenFuncCallDone(aCall)
	//这里存在一个问题，如果client没有等收到所有的响应，就关闭。会导致这里向一个已经closed的chan发数据，导致panic。然后被上面的recove住
}

func (this *TServerConn) ReadRequest() (service *TService, mtype *TMethodType, req TRequestHeader, argv, replyv reflect.Value, keepReading bool, err error) {
	service, mtype, req, keepReading, err = this.ReadRequestHeader()
	if err != nil {
		if !keepReading {
			return
		}
		this.codec.ReadRequestBody(nil)
		return
	}

	if req.IsPing() {
		return
	}

	//根据method知道参数类型，然后根据参数类型生成参数对象
	argIsValue := false //生成参数对象时，都是生成的指针。所以如果method需要值类型参数，在返回前需要处理一下（indirect）
	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType.Elem())
	} else {
		argv = reflect.New(mtype.ArgType)
		argIsValue = true
	}

	if err = this.codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}
	if argIsValue {
		argv = argv.Elem()
	}

	replyv = reflect.New(mtype.ReplyType.Elem())
	return
}

func (this *TServerConn) ReadRequestHeader() (service *TService, mtype *TMethodType, reqHeader TRequestHeader, keepReading bool, err error) {

	reqHeader = TRequestHeader{}
	err = this.codec.ReadRequestHeader(&reqHeader)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully.  If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	if reqHeader.IsPing() {
		return
	}

	service, mtype, err = this.server.getSeviceMethod(reqHeader.ServiceMethod)

	return
}

func (this *TServerConn) makeRespHeader(reqHeader TRequestHeader, reply interface{}, errmsg string) TResponseHeader {
	respHeader := TResponseHeader{}
	// Encode the response header
	respHeader.ServiceMethod = reqHeader.ServiceMethod

	if reqHeader.IsPing() {
		respHeader.Flag |= ResponseFlagPing
	}
	if errmsg != "" {
		respHeader.ErrorMsg = errmsg
		reply = nil
	}
	if reply == nil {
		respHeader.Flag |= ResponseFlagNoData
	}

	respHeader.Seq = reqHeader.Seq
	return respHeader
}

func (this *TServerConn) SendResponse(reqHeader TRequestHeader, reply interface{}, errmsg string) {
	respHeader := this.makeRespHeader(reqHeader, reply, errmsg)
	this.sendingMutex.Lock()
	err := this.codec.WriteResponse(&respHeader, reply)
	if err != nil {
		this.logger.Fatal("rpc: writing response:%v", err)
	} else {
		err = this.codec.Flush()
		if err != nil {
			this.logger.Fatal("rpc: flush:%v", err)
		}
	}
	this.sendingMutex.Unlock()
	this.logger.Debug("sendResponse for req:%s %v done", reqHeader.ServiceMethod, reqHeader)

	this.pendingWg.Done()
}

func (this *TServerConn) SendGroupResponse(respGroup []TServerGroupResponse) {
	this.sendingMutex.Lock()
	for _, resp := range respGroup {
		err := this.codec.WriteResponse(&(resp.resp), resp.call.Reply.Interface())
		if err != nil {
			this.logger.Fatal("rpc: writing response:%v", err)
		}
		if resp.call.needSendRespWhenDone() {
			FreeFuncCall(resp.call)
		}
	}

	err := this.codec.Flush()
	if err != nil {
		this.logger.Fatal("rpc: flush:%v", err)
	}
	this.sendingMutex.Unlock()
	this.logger.Debug("rpc send groupRespone len = %d", len(respGroup))

	for _, _ = range respGroup {
		this.pendingWg.Done()
	}
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
