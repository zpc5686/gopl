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

package rpc

import (
	log "babeltime.com/log4go"
	"reflect"
	"time"
)

var gLogger = log.NewLogger()

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

var DefClientConfig TClientConfig = TClientConfig{
	MaxWaitCall:    1024,
	MaxRetryNum:    1,
	RequestTimeout: time.Second * 3,
	PingInterval:   time.Second * 60,
}

const (
	PING_METHOD = "ping"
)

const (
	RequestFlagNull TypeRequestFlag = 0
	RequestFlagPing                 = 1 << iota
	RequestFlagDummyReturn
)

const (
	ResponseFlagNull TypeResponseFlag = 0
	ResponseFlagPing                  = 1 << iota
	ResponseFlagNoData
)

type TypeRequestFlag uint32
type TypeResponseFlag uint32

type FDispatchCall func(aCall *TFuncCall) (bool, error)
type FOnClientInValid func(aClient *TClient) //当client发现所有的服务都不可用的回调

type TFuncCall struct {
	MethodName string
	Func       reflect.Value
	Rcvr       reflect.Value
	Arg        reflect.Value
	Reply      reflect.Value
	Ret        []reflect.Value
	Header     TRequestHeader
	Done       chan *TFuncCall
}

type TClientConfig struct {
	MaxWaitCall    uint32        //一个服务地址上最大等待的请求数
	MaxRetryNum    uint32        //重试次数，＝0表示不重试
	RequestTimeout time.Duration //超时时间，只在同步请求时使用
	PingInterval   time.Duration //ping间隔
}

type TServerConfig struct {
	ServerIdleTimeout time.Duration //服务器等待请求的超时时间，如果超过这个时间，就会关闭连接
	MaxWaitResp       uint32        //每个连接上，等待写会的响应的最大个数
}

type TRequestHeader struct {
	ServiceMethod string
	Seq           uint64
	Flag          TypeRequestFlag //是否是ping，是否需要立刻返回等
}

type TResponseHeader struct {
	ServiceMethod string
	Seq           uint64
	ErrorMsg      string
	Flag          TypeResponseFlag
}

type TMethodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

type TService struct {
	name   string
	rcvr   reflect.Value
	typ    reflect.Type
	method map[string]*TMethodType
}

func (this *TRequestHeader) IsPing() bool {
	return this.Flag&RequestFlagPing > 0
}

func (this *TRequestHeader) DummyReturn() bool {
	return this.Flag&RequestFlagDummyReturn > 0
}

func (this *TResponseHeader) IsPing() bool {
	return this.Flag&ResponseFlagPing > 0
}

func (this *TResponseHeader) HasData() bool {
	return this.Flag&ResponseFlagNoData == 0
}

func (this *TFuncCall) Reset() {
	*this = TFuncCall{}
}

func (this *TFuncCall) needSendRespWhenDone() bool {
	return this.Done != nil
}

//为了在内部某些地方打印日志时，能打印外部某个flag
type IGetStringFlag interface {
	GetStringFlag() string
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
