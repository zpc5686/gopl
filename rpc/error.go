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

const (
	ErrorTypeOk = iota
	ErrorTypeUnknown
	ErrorTypeNoServer
	ErrorTypeServerPause
	ErrorTypeServerConnectFailed
	ErrorTypeChanFull
	ErrorTypeInputCallChanClosed
	ErrorTypeShutdown
	ErrorTypeRequestTimeout
)

type TRpcError struct {
	typ int
	msg string
}

var (
	ErrNoServer            = NewRpcError(ErrorTypeNoServer, "not connect to any server")
	ErrServerPause         = NewRpcError(ErrorTypeServerPause, "server pause")
	ErrServerConnectFailed = NewRpcError(ErrorTypeServerConnectFailed, "failed when connecting to server")
	ErrChanFull            = NewRpcError(ErrorTypeChanFull, "call chan full")
	ErrInputCallChanClosed = NewRpcError(ErrorTypeInputCallChanClosed, "client input call chan closed")
	ErrShutdown            = NewRpcError(ErrorTypeShutdown, "connection is shut down")
	ErrRequestTimeout      = NewRpcError(ErrorTypeRequestTimeout, "request time out")
)

func NewRpcError(aTyp int, aMsg string) TRpcError {
	return TRpcError{aTyp, aMsg}
}

func IsRpcError(err interface{}) bool {
	switch err.(type) {
	case *TRpcError:
		return true
	case TRpcError:
		return true
	default:
		return false
	}
}

func IsRpcErrorType(err interface{}, aType int) bool {
	switch e := err.(type) {
	case *TRpcError:
		return e.typ == aType
	case TRpcError:
		return e.typ == aType
	default:
		return false
	}
	return false
}

func canRetry(err interface{}) bool {
	switch e := err.(type) {
	case *TRpcError:
		return (*e).CanRetry()
	case TRpcError:
		return e.CanRetry()
	default:
		return false
	}
	return false
}

func (this TRpcError) CanRetry() bool {
	return this.typ == ErrorTypeServerConnectFailed ||
		this.typ == ErrorTypeChanFull ||
		this.typ == ErrorTypeShutdown
}

func (this TRpcError) Error() string {
	return this.msg
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
