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
	"babeltime.com/encoding/amf"
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"io"
)

type FServerCodecFactory func(aIo io.ReadWriteCloser) IServerCodec
type FClientCodecFactory func(aIo io.ReadWriteCloser) IClientCodec

type IServerCodec interface {
	ReadRequestHeader(*TRequestHeader) error
	ReadRequestBody(interface{}) error
	WriteResponse(*TResponseHeader, interface{}) error
	Flush() error

	Close() error
}

type IClientCodec interface {
	WriteRequest(*TRequestHeader, interface{}) error
	ReadResponseHeader(*TResponseHeader) error
	ReadResponseBody(interface{}) error
	Flush() error

	Close() error
}

func NewServerCodec(aRwc io.ReadWriteCloser) IServerCodec {
	return NewGobCodec(aRwc)
}

func NewClientCodec(aRwc io.ReadWriteCloser) IClientCodec {
	return NewGobCodec(aRwc)
}

func NewWebServerCodec(aRwc io.ReadWriteCloser) IServerCodec {
	return NewAmfCodec(aRwc)
}

func NewWebClientCodec(aRwc io.ReadWriteCloser) IClientCodec {
	return NewAmfCodec(aRwc)
}

func NewGobCodec(aRwc io.ReadWriteCloser) *TGobCodec {
	buf := bufio.NewWriter(aRwc)

	return &TGobCodec{
		rwc:    aRwc,
		dec:    gob.NewDecoder(aRwc),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}
}

type TGobCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

func (this *TGobCodec) ReadRequestHeader(aReqHeader *TRequestHeader) error {
	return this.dec.Decode(aReqHeader)
}

func (this *TGobCodec) ReadRequestBody(aBody interface{}) error {
	return this.dec.Decode(aBody)
}

func (this *TGobCodec) WriteResponse(aResHeader *TResponseHeader, aBody interface{}) (err error) {
	if err = this.enc.Encode(aResHeader); err != nil {
		if this.encBuf.Flush() == nil {
			//编码出错，就先把缓冲中的数据先刷出去，然后关闭连接，如果flush出错，估计就是网络出问题了，也不用close了
			gLogger.Fatal("gob encoding response header failed:%s", err.Error())
			this.Close()
		}
		return
	}

	if aBody == nil {
		return nil
	}

	if err = this.enc.Encode(aBody); err != nil {
		if this.encBuf.Flush() == nil {
			gLogger.Fatal("gob encoding response body failed:%s", err.Error())
			this.Close()
		}
		return
	}
	return nil
}

func (this *TGobCodec) Flush() error {
	return this.encBuf.Flush()
}

func (this *TGobCodec) Close() error {
	if this.closed {
		gLogger.Warning("connectio already closed")
		return nil
	}
	this.closed = true
	return this.rwc.Close()
}

func (this *TGobCodec) WriteRequest(aReqHeader *TRequestHeader, aBody interface{}) (err error) {
	if err = this.enc.Encode(aReqHeader); err != nil {
		return
	}

	if aBody == nil {
		return nil
	}

	if err = this.enc.Encode(aBody); err != nil {
		return
	}
	return nil
}

func (this *TGobCodec) ReadResponseHeader(aResHeader *TResponseHeader) error {
	return this.dec.Decode(aResHeader)
}

func (this *TGobCodec) ReadResponseBody(aBody interface{}) error {
	return this.dec.Decode(aBody)
}

type TAmfCodec struct {
	rwc    io.ReadWriteCloser
	writer *bufio.Writer
	closed bool
}

func NewAmfCodec(aRwc io.ReadWriteCloser) *TAmfCodec {
	buf := bufio.NewWriter(aRwc)

	return &TAmfCodec{
		rwc:    aRwc,
		writer: buf,
	}
}

func (this *TAmfCodec) ReadRequestHeader(aReqHeader *TRequestHeader) error {
	data := make([]byte, 1024)
	l, err := this.rwc.Read(data)
	gLogger.Debug("read length = %d", l)
	if err != nil {
		gLogger.Debug("read err:%v", err)
		return err
	}
	gLogger.Debug("data = %s", string(data[:l]))
	jsonData, err := AmfToJson(data[:l])
	if err != nil {
		gLogger.Debug("amf to json err:%v", err)
		return err
	}
	gLogger.Debug("json is %s", string(jsonData))
	return amf.Decode(this.rwc, binary.BigEndian, aReqHeader)
}

func (this *TAmfCodec) ReadRequestBody(aBody interface{}) error {
	return amf.Decode(this.rwc, binary.BigEndian, aBody)
}

func (this *TAmfCodec) WriteResponse(aResHeader *TResponseHeader, aBody interface{}) (err error) {
	if err = amf.Encode(this.writer, binary.BigEndian, aResHeader); err != nil {
		if this.writer.Flush() == nil {
			//编码出错，就先把缓冲中的数据先刷出去，然后关闭连接，如果flush出错，估计就是网络出问题了，也不用close了
			gLogger.Fatal("amf encoding response header failed:%s", err.Error())
			this.Close()
		}
		return
	}

	if aBody == nil {
		return nil
	}

	if err = amf.Encode(this.writer, binary.BigEndian, aBody); err != nil {
		if this.writer.Flush() == nil {
			gLogger.Fatal("amf encoding response body failed:%s", err.Error())
			this.Close()
		}
		return
	}
	return nil
}

func (this *TAmfCodec) Flush() error {
	return this.writer.Flush()
}

func (this *TAmfCodec) Close() error {
	if this.closed {
		gLogger.Warning("connectio already closed")
		return nil
	}
	this.closed = true
	return this.rwc.Close()
}

func (this *TAmfCodec) WriteRequest(aReqHeader *TRequestHeader, aBody interface{}) (err error) {
	if err = amf.Encode(this.writer, binary.BigEndian, aReqHeader); err != nil {
		return
	}

	if aBody == nil {
		return nil
	}

	if err = amf.Encode(this.writer, binary.BigEndian, aBody); err != nil {
		return
	}
	return nil
}

type ICodec interface {
	Decode([]byte, interface{}) error
	Encode(interface{}) ([]byte, error)
}

var gJsonCodec ICodec = &TCodecJson{}
var gAmfCodec ICodec = &TCodecAmf{}

type TCodecJson struct {
}

func (this *TCodecJson) Decode(aEncodedData []byte, aObj interface{}) error {
	if len(aEncodedData) <= 0 {
		return nil
	}
	return json.Unmarshal(aEncodedData, aObj)
}

func (this *TCodecJson) Encode(aObj interface{}) ([]byte, error) {
	return json.Marshal(aObj)
}

type TCodecAmf struct {
}

func (this *TCodecAmf) Decode(aEncodedData []byte, aObj interface{}) error {
	buffer := bytes.NewBuffer(aEncodedData)
	return amf.Decode(buffer, binary.BigEndian, aObj)
}

func (this *TCodecAmf) Encode(aObj interface{}) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	err := amf.Encode(buffer, binary.BigEndian, aObj)
	if err != nil {
		return nil, err
	} else {
		return buffer.Bytes(), nil
	}
}

func AmfToJson(data []byte) ([]byte, error) {
	mp := make(map[string]interface{})
	err := gAmfCodec.Decode(data, &mp)
	if err != nil {
		gLogger.Fatal("amf decode err:%s", err.Error())
		return nil, err
	}
	jsondata, err := gJsonCodec.Encode(&mp)
	if err != nil {
		gLogger.Fatal("json encode[%+v] err:%s", mp, err.Error())
		return nil, err
	}
	return jsondata, nil
}

func (this *TAmfCodec) ReadResponseHeader(aResHeader *TResponseHeader) error {
	return amf.Decode(this.rwc, binary.BigEndian, aResHeader)
}

func (this *TAmfCodec) ReadResponseBody(aBody interface{}) error {
	return amf.Decode(this.rwc, binary.BigEndian, aBody)
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
