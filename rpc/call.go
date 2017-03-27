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
   TCall:记录一个rpc调用，并通过chan来通知调用结果

*/

package rpc

import (
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

/*
关于请求超时的处理，目前采用的方案:

1）异步调用不做超时处理
2）同步请求在请求的最外层（包住了多次尝试）设置一个超时timer

调用层可能因为超时等原因放弃请求，应该避免被放弃的请求被发送出去
我们使用cancel标记这个call是否被放弃
如果在call被发送routine（RoutineWriteRequst）发送之前，timer到期，timer中会将cancel加1，此call就不用处理
这个措施不能完全避免到期的call被发送出去，也不能让超时的call的done()不被调用
要实现绝对安全的cancel需要付出的代价太大
*/

var gCallPool *sync.Pool

type TCall struct {
	SeviceMethod string
	Args         interface{}
	Reply        interface{}
	Extra        interface{} //和请求相关的一些额外信息，这个信息不会发送出去。只是为了记录一些相关信息，在发送请求，收到响应等阶段方便外部使用
	Error        error
	Done         chan *TCall
	StartTime    time.Time
	Cancel       int32
}

func init() {
	gCallPool = &sync.Pool{
		New: func() interface{} {
			return new(TCall)
		},
	}
}

func NewCall() *TCall {
	call := gCallPool.Get().(*TCall)
	call.Cancel = 0
	call.StartTime = time.Now()
	return call
}

func FreeCall(aCall *TCall) {
	//Args,Replay,Extra外部也可能有pool，这里早点释放对其的引用
	aCall.Reset()
	gCallPool.Put(aCall)
}

func WhenCallDone(aCall *TCall) {

	//需要通知done消息的才会调用done函数，否则可以free了（外面不需要）
	if aCall.needDoneNotify() {
		aCall.done()
	} else {
		FreeCall(aCall)
	}
}

func (this *TCall) Reset() {
	*this = TCall{}
}

func (this *TCall) CancelCall() {
	atomic.AddInt32(&this.Cancel, 1)
}

func (this *TCall) IsCanceled() bool {
	return (atomic.LoadInt32(&this.Cancel) > 0)
}

func (this *TCall) done() {
	if this.IsCanceled() {
		FreeCall(this)
		return
	}
	method := this.SeviceMethod
	defer func() {
		if r := recover(); r != nil {
			gLogger.Fatal("call chan has close method:%s, err:%v, stack:%s", method, r, debug.Stack())
			FreeCall(this)
		}
	}()
	select {
	case this.Done <- this:
		// gLogger.Debug("method:%s %v cost:%f", this.SeviceMethod, this.Args, time.Since(this.StartTime).Seconds())
		// ok
	default:
		//需要外部保证chan有足够的空间，这里不希望阻塞
		gLogger.Fatal("discarding Call reply due to insufficient Done chan capacity")
	}
}

func (this *TCall) needDoneNotify() bool {
	return this.Done != nil
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
