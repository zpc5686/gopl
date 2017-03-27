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
   TClientConn:负责一个和server的连接

*/

package rpc

import (
	log "babeltime.com/log4go"
	"errors"
	"io"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

var pingRequestHreader TRequestHeader = TRequestHeader{
	ServiceMethod: PING_METHOD,
	Seq:           0,
	Flag:          RequestFlagPing,
}

/*
关于锁的问题
 reqMutex 不只是要保护requestHeader字段，也保护了发送请求的操作（包含对的pending的修改。详见RoutineReadResponse）
 mutex保护seq, pengding, closeing shutdown。其实保护的是接收响应的操作
*/
type TClientConn struct {
	parent        *TClient
	addr          string
	codec         IClientCodec
	inputCallChan chan *TCall
	exitNotify    chan bool
	mutex         sync.Mutex //保护下面的变量
	seq           uint64
	pending       map[uint64]*TCall
	closing       bool //用户调用了Close
	shutdown      bool //服务器断开连接
	logger        *log.Logger
}

type TClientGroupRequest struct {
	req  TRequestHeader
	call *TCall
}

func NewClientConn(aAddr string, aParent *TClient, aInputCallChan chan *TCall) *TClientConn {

	conn := &TClientConn{
		parent:        aParent,
		addr:          aAddr,
		inputCallChan: aInputCallChan,
		exitNotify:    make(chan bool, 1),
		pending:       make(map[uint64]*TCall),
	}

	return conn
}

func (this *TClientConn) Start(aCodecFactory FClientCodecFactory) error {

	// tcpAddr, err := net.ResolveTCPAddr("tcp4", this.addr)
	// if err != nil {
	//     return err
	// }
	// netConn, err := net.DialTCP("tcp4", nil, tcpAddr)
	// if err != nil {
	//     return err
	// }

	netType, addr, err := getNetAddr(this.addr, false)
	if err != nil {
		return err
	}

	netConn, err := net.Dial(netType, addr)
	if err != nil {
		return err
	}

	logger := log.NewLogger()
	logger.AddInfo("remoteAddr", netConn.RemoteAddr().String())
	this.logger = logger
	this.codec = aCodecFactory(netConn)

	go this.RoutineReadResponse()
	go this.RoutineWriteRequst()
	return nil
}

func (this *TClientConn) RoutineWriteRequst() {
	defer func() {
		if r := recover(); r != nil {
			this.logger.Fatal("RoutineWriteRequst erro,  serverAddr:%s, recover:%v, stack:\n%s", this.addr, r, debug.Stack())
		}
	}()

	this.logger.Info("RoutineWriteRequst start for %s", this.addr)
	var err error

	pingTimer := time.NewTimer(this.parent.config.PingInterval)

L_for:
	for err == nil {
		select {
		case call, ok := <-this.inputCallChan:
			if !ok {
				err = ErrInputCallChanClosed
				this.logger.Warning("inputCallChan closed")
				break L_for
			}

			//拿到call后，如果conn还能工作，就构造请求发出去，否则返回错误
			err = this.sendCall(call)

		case <-pingTimer.C:
			this.logger.Debug("send ping")
			err = this.sendPing()

		case <-this.exitNotify:
			this.logger.Warning("get exit notify")
			err = ErrShutdown
			break L_for

		}
		pingTimer.Reset(this.parent.config.PingInterval)
	}

	this.logger.Fatal("something err on RoutineWriteRequst. stop service. start clean up")
}

func (this *TClientConn) RoutineReadResponse() {
	defer func() {
		if r := recover(); r != nil {
			this.logger.Fatal("RoutineReadResponse erro,  serverAddr:%s, recover:%v, stack:\n%s", this.addr, r, debug.Stack())
		}
	}()

	this.logger.Info("RoutineReadResponse start for %s", this.addr)
	var err error
	var responseHeader TResponseHeader
	for err == nil {
		responseHeader = TResponseHeader{}
		err = this.codec.ReadResponseHeader(&responseHeader)
		if err != nil {
			this.logger.Fatal("ReadResponseHeader failed. err:%s", err.Error())
			break
		}

		if responseHeader.IsPing() {
			this.logger.Debug("get response for ping")
			continue
		}

		seq := responseHeader.Seq
		this.mutex.Lock()
		call := this.pending[seq]
		delete(this.pending, seq)
		this.mutex.Unlock()

		switch {
		case call == nil:
			/*
			   收到响应，但是找不到对应的call。应该是WriteRequest部分失败，call已经被删掉了
			   得到的响应应该是服务告诉我们收到的一个错误的请求包
			   我们应该读到这个响应，然后无视它
			*/
			this.logger.Fatal("not found call for request:%s %v", responseHeader.ServiceMethod, responseHeader)
			if responseHeader.HasData() {
				err = this.codec.ReadResponseBody(nil)
				if err != nil {
					errMsg := "reading error body: " + err.Error()
					this.logger.Warning("%s", errMsg)
					err = errors.New(errMsg)
				}
			}

		case responseHeader.ErrorMsg != "":
			call.Error = NewRpcError(ErrorTypeUnknown, responseHeader.ErrorMsg) //业务错误，就不重试了

			if responseHeader.HasData() {
				err = this.codec.ReadResponseBody(nil)
				if err != nil {
					errMsg := "reading error body: " + err.Error()
					this.logger.Warning("%s", errMsg)
					err = errors.New(errMsg)
				}
			}
			WhenCallDone(call)

		default:

			if responseHeader.HasData() {
				err = this.codec.ReadResponseBody(call.Reply)
				this.logger.Debug("readResponse for req:%s %v", call.SeviceMethod, call.Args)
				if err != nil {
					errMsg := "reading body " + err.Error()
					this.logger.Warning("%s", errMsg)
					call.Error = errors.New(errMsg)
				}
			}
			WhenCallDone(call)
		}
	}
	this.logger.Fatal("something err:%s on RoutineReadResponse. stop service. start clean up", err.Error())
	this.onShutDown(err)
}

func (this *TClientConn) makeRequestHeader(aCall *TCall) TRequestHeader {
	//担心一直有残留的call没有得到响应，这里加个告警日志
	pendingNum := len(this.pending)
	if pendingNum > 100 {
		this.logger.Debug("there %d pending call", pendingNum)
	}
	seq := this.seq
	this.seq++
	this.pending[seq] = aCall

	requestHeader := TRequestHeader{}
	requestHeader.Seq = seq
	requestHeader.ServiceMethod = aCall.SeviceMethod
	requestHeader.Flag = 0
	if aCall.Reply == nil {
		requestHeader.Flag |= RequestFlagDummyReturn //reply为nil说明不需要返回值
	}

	if aCall.SeviceMethod == PING_METHOD {
		requestHeader.Flag |= RequestFlagPing
		this.logger.Debug("send ping ")
	}
	return requestHeader
}

func (this *TClientConn) sendCall(aCall *TCall) (err error) {
	this.mutex.Lock()
	if this.shutdown || this.closing {
		this.logger.Warning("connection shutdown after get call, return err")
		aCall.Error = ErrShutdown
		this.mutex.Unlock()
		WhenCallDone(aCall)
		err = ErrShutdown
		return
	}

	var groupRequest []TClientGroupRequest
	reqHeader := this.makeRequestHeader(aCall)
	req := TClientGroupRequest{
		req:  reqHeader,
		call: aCall,
	}
	groupRequest = append(groupRequest, req)
	looping := true
	for i := 0; i < UNITE_PACKAGE_NUM && looping; i++ {
		select {
		case call, ok := <-this.inputCallChan:
			if !ok {
				this.mutex.Unlock()
				return ErrShutdown
			}
			reqHeader := this.makeRequestHeader(call)
			req := TClientGroupRequest{
				req:  reqHeader,
				call: call,
			}
			groupRequest = append(groupRequest, req)
		default:
			looping = false
		}
	}
	this.mutex.Unlock()

	err = this.sendGroupCall(groupRequest)
	return
}

func (this *TClientConn) sendGroupCall(groupRequest []TClientGroupRequest) (err error) {
	for _, req := range groupRequest {
		this.logger.Debug("write request:%s %v", req.call.SeviceMethod, req.call.Args)
		err = this.codec.WriteRequest(&(req.req), req.call.Args)
		if err != nil {
			break
		}
	}
	this.logger.Debug("write groupRequest len = %d", len(groupRequest))
	if err == nil {
		err = this.codec.Flush()
	}
	if err != nil {
		for _, req := range groupRequest {
			seq := req.req.Seq
			this.mutex.Lock()
			aCall := this.pending[seq] //此处不是无用操作。是从pending中查一下对应的call是否还存在（可能在RoutineReadResonse中删除了）
			delete(this.pending, seq)
			this.mutex.Unlock()
			if aCall != nil {
				this.logger.Fatal("WriteRequest failed when sendCall. err:%s", err.Error())
				aCall.Error = err //TODO 这样的错误可重试吗
				WhenCallDone(aCall)
			}
		}
	}
	return
}

func (this *TClientConn) sendPing() (err error) {
	err = this.codec.WriteRequest(&pingRequestHreader, nil)
	if err != nil {
		this.logger.Fatal("send ping request failed:%s", err.Error())
	} else {
		err = this.codec.Flush()
	}
	return
}

func (this *TClientConn) onShutDown(aErr error) {

	this.logger.Info("onShutDown addr:%s, err:%v", this.addr, aErr)
	this.mutex.Lock()
	if this.shutdown {
		this.logger.Warning("already call shutDown")
		this.mutex.Unlock()
		return
	}
	this.shutdown = true
	closing := this.closing
	if aErr == io.EOF {
		if closing {
			aErr = ErrShutdown
		} else {
			aErr = io.ErrUnexpectedEOF
		}
	}

	this.logger.Warning("there %d pending request", len(this.pending))
	for seq, call := range this.pending {
		this.logger.Fatal("call seq:%d failed because client down", seq)
		call.Error = aErr
		WhenCallDone(call)
	}
	this.exitNotify <- true
	this.mutex.Unlock()

	this.parent.onConnShutdown(this)
}

func (this *TClientConn) Close() error {
	this.mutex.Lock()
	if this.closing {
		this.mutex.Unlock()
		return ErrShutdown
	}
	this.closing = true
	this.mutex.Unlock()
	return this.codec.Close()
}

func getNetAddr(addr string, removeFileWhenUnix bool) (string, string, error) {
	subString := strings.Split(addr, ":")
	if len(subString) < 2 {
		return "tcp4", addr, nil
	}
	port, err := strconv.Atoi(subString[1])
	if err != nil {
		return "tcp4", addr, nil
	}
	//如果addr的port小于等于0，就使用unix
	if port <= 0 {
		if removeFileWhenUnix {
			_, err := os.Stat(subString[0])
			if err != nil && os.IsNotExist(err) {
				return "unix", subString[0], nil
			}
			err = os.Remove(subString[0])
			if err != nil {
				return "", "", err
			}
		}
		return "unix", subString[0], nil
	}
	return "tcp4", addr, nil
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
