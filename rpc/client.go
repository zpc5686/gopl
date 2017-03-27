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
   对一个rpc服务的客户端。内部支持多个服务地址。
   服务器地址需要外部指定，如果要做服务器地址管理（类似phpproxy那样的名字服务功能），可以另封装一个单独的服务
   使用时，应该一种服务对应一个TClient对象。

每个连接并不等待上一个请求得到响应再发送下一个请求。这样可以充分利用到tcp的全双工特性，用比较少的连接得到比较大的吞度量

三类连接
    1）还没有创建的连接
    2）空闲的（没有未完成请求的）
    3）正在提供服务的

选择连接的优先级
    1）空闲连接
    2）创建新连接
        找到所有还可以创建连接的服务地址，按权重随机选择一个，创建连接
    3）正在提供服务的
        找等待请求最少的





*/

package rpc

import (
	log "babeltime.com/log4go"
	"container/list"
	"fmt"
	"math/rand"
	"sync"
	//	"time"
)

const (
	REMOTE_SERVER_STATUS_NORM           = iota //正常状态
	REMOTE_SERVER_STATUS_PAUSE                 //服务正在更新，暂时不能用
	REMOTE_SERVER_STATUS_CONNECT_FAILED        //连接失败
)

type TServerInfo struct {
	Addr       string      //服务器地址
	Weight     int         //权重
	MaxConnNum int         //最大连接数
	CallChan   chan *TCall //等待处理的call
	ConnList   *list.List
	Status     int
}

//用于给外部返回服务信息用
type TServerInfoReport struct {
	Addr       string //服务器地址
	Weight     int    //权重
	MaxConnNum int    //最大连接数
	CurConnNum int
	Status     int
}

type TClient struct {
	serverName      string //一个服务一个client，记录服务名字
	mu              sync.RWMutex
	mapServerInfo   map[string]*TServerInfo
	totalWeight     int
	logger          *log.Logger
	config          TClientConfig
	codecFactory    FClientCodecFactory
	onClientInvalid FOnClientInValid
}

func NewClient(aServiceName string, aConfig *TClientConfig, aCodecFactory FClientCodecFactory) *TClient {

	client := &TClient{
		serverName:    aServiceName,
		mapServerInfo: make(map[string]*TServerInfo),
		logger:        log.NewLogger(),
		codecFactory:  aCodecFactory,
	}

	if aConfig == nil {
		client.config = DefClientConfig
	} else {
		client.config = *aConfig
	}

	client.logger.Info("new client for service:%s, config:%v", aServiceName, client.config)
	return client
}

func (this *TClient) GetServiceName() string {
	return this.serverName
}

func (this *TClient) SetOnInvalidCallback(aCallback FOnClientInValid) {
	this.onClientInvalid = aCallback
}

func (this *TClient) AddServer(aAddr string, aWeight int, aMaxConnNum int) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if serverInfo, ok := this.mapServerInfo[aAddr]; ok {
		serverInfo.Weight = aWeight
		serverInfo.MaxConnNum = aMaxConnNum
		this.logger.Info("%s already exist, update info")
	} else {
		this.mapServerInfo[aAddr] = &TServerInfo{
			Addr:       aAddr,
			Weight:     aWeight,
			MaxConnNum: aMaxConnNum,
			CallChan:   make(chan *TCall, this.config.MaxWaitCall),
			ConnList:   list.New(),
		}
		this.logger.Info("add server addr. name:%s, addr:%s, weight:%d, maxConnNum:%d", this.serverName, aAddr, aWeight, aMaxConnNum)
	}
	this.updateTotalWeight()
}

func (this *TClient) updateTotalWeight() {
	this.totalWeight = 0
	for _, serverInfo := range this.mapServerInfo {
		if serverInfo.Status == REMOTE_SERVER_STATUS_NORM {
			this.totalWeight += serverInfo.Weight
		}
	}
	this.logger.Info("update totalWeight:%d", this.totalWeight)
}

func (this *TClient) SendCall(aCall *TCall) {

	this.logger.Debug("SendCall req:%s", aCall.SeviceMethod)
	serverInfo, err := this.chooseServerForReq(true)

	if err != nil {
		this.logger.Fatal("no server for req:%s", aCall.SeviceMethod)
		aCall.Error = err
		WhenCallDone(aCall)
		return
	}

	//TODO 为了打印个uid，这个折腾好吗？
	if flag, ok := aCall.Args.(IGetStringFlag); ok {
		this.logger.Trace("rpcCall:%s, server:%s, flag:%s", aCall.SeviceMethod, serverInfo.Addr, flag.GetStringFlag())
	} else {
		this.logger.Trace("rpcCall:%s, server:%s", aCall.SeviceMethod, serverInfo.Addr)
	}

	select {
	case serverInfo.CallChan <- aCall:
		//nothing to do
	default: //满了
		this.logger.Warning("server:%s call chan full", serverInfo.Addr)
		aCall.Error = ErrChanFull
		WhenCallDone(aCall)
	}
}

/*
异步调用接口，不做重试
如果需要得到执行完后的通知，需要自己传入一个非nil的aDoneChan
并且需要自己FreeCall

return
	aDoneChan != nil 时返回Call对象
	aDoneChan == nil 是返回nil
*/
func (this *TClient) AsyncCall(aServiceMethod string, aArgs interface{}, aReply interface{}, aDoneChan chan *TCall) *TCall {

	call := NewCall()
	call.SeviceMethod = aServiceMethod
	call.Args = aArgs
	call.Reply = aReply
	call.Done = aDoneChan

	this.SendCall(call)

	if aDoneChan == nil {
		return nil
	} else {
		return call
	}
}

func (this *TClient) SyncCall(aServiceMethod string, aArgs interface{}, aReply interface{}) error {

	call := NewCall()
	call.SeviceMethod = aServiceMethod
	call.Args = aArgs
	call.Reply = aReply
	call.Done = make(chan *TCall, 1)

	var tryNum uint32
	//	timer := time.NewTimer(this.config.RequestTimeout) //TODO 是不是要考虑一下timer的效率问题
	//	for tryNum <= this.config.MaxRetryNum {

	this.SendCall(call)
	select {
	case call = <-call.Done:
		//
		//	case <-timer.C:
		//		this.logger.Fatal("method:%s %v, timeout", aServiceMethod, aArgs)
		//		close(call.Done)
		//		call.CancelCall()
		//		//此处不能freeCall，这个Call可能还被某个clientConn持有
		//		return ErrRequestTimeout
	}

	if call.Error == nil {
		FreeCall(call)
		return nil
	}
	//		if canRetry(call.Error) {
	//			tryNum++
	//			this.logger.Warning("call %s failed. err:%s retry:%d", aServiceMethod, call.Error.Error(), tryNum)
	//			continue
	//		}
	//		break
	//	}

	err := call.Error

	FreeCall(call)

	this.logger.Warning("call method:%s %v failed after %d retry", aServiceMethod, aArgs, tryNum)
	return err
}

func (this *TClient) addConn(aServerInfo *TServerInfo) *TClientConn {
	conn := NewClientConn(aServerInfo.Addr, this, aServerInfo.CallChan)
	aServerInfo.ConnList.PushBack(conn)
	this.logger.Info("server:%s add connection. curNum:%d", aServerInfo.Addr, aServerInfo.ConnList.Len())
	return conn
}

func (this *TClient) removeConn(aServerInfo *TServerInfo, conn *TClientConn) int {
	for e := aServerInfo.ConnList.Back(); e != nil; e = e.Prev() {
		if e.Value == conn {
			aServerInfo.ConnList.Remove(e)
			return aServerInfo.ConnList.Len()
		}
	}
	this.logger.Warning("server:%s removeConn err: not find connection", aServerInfo.Addr)
	return aServerInfo.ConnList.Len()
}

func (this *TClient) getServerInfo() (*TServerInfo, error) {

	err := ErrNoServer //默认错误类型

	var randNum = 0
	if this.totalWeight > 0 {
		randNum = rand.Intn(this.totalWeight)
	}
	var sum int = 0
	for _, serverInfo := range this.mapServerInfo {
		if serverInfo.Status == REMOTE_SERVER_STATUS_PAUSE {
			err = ErrServerPause
			//如果有server在更新中，就返回服务更新中的错误。没有考虑有其他错误类型。如果要添加其他错误类型，需要修改这里
			continue
		}
		sum += serverInfo.Weight
		if sum >= randNum {
			return serverInfo, nil
			break
		}
	}
	return nil, err
}

func (this *TClient) chooseServerForReq(aCreateConnWhenNeed bool) (aServerInfo *TServerInfo, err error) {

	this.mu.RLock()

	serverInfo, err := this.getServerInfo()

	if serverInfo == nil {
		this.mu.RUnlock()
		this.logger.Fatal("not connect to any server:%s", this.serverName)
		return nil, err
	}

	connNum := serverInfo.ConnList.Len()
	//当有连接存在，且 （ 没有积压请求，或者连接数已满 ）时，直接返回，否则就需要考虑新建连接
	if connNum > 0 && (len(serverInfo.CallChan) <= 0 || connNum >= serverInfo.MaxConnNum) {
		this.logger.Debug("use server:%s, connNum:%d", serverInfo.Addr, connNum)
		this.mu.RUnlock()
		return serverInfo, nil
	}

	this.logger.Debug("need to create new connection to server:%s_%s", this.serverName, serverInfo.Addr)

	this.mu.RUnlock()

	this.mu.Lock()
	connNum = serverInfo.ConnList.Len() //重新获取一下连接数，避免2个routine重复创建

	var conn *TClientConn
	//当连接为空或者还有未处理完的请求且连接数不满时，创建新请求
	if connNum == 0 || (len(serverInfo.CallChan) > 0 && connNum < serverInfo.MaxConnNum) {
		this.logger.Info("server:%s_%s no connection or has waiting call. connNum:%d, try create new connection", this.serverName, serverInfo.Addr, connNum)
		conn = this.addConn(serverInfo)
	}
	this.mu.Unlock()
	if conn != nil {
		err = conn.Start(this.codecFactory)
		if err != nil {
			this.mu.Lock()
			connNum = this.removeConn(serverInfo, conn)
			if connNum == 0 {
				serverInfo.Status = REMOTE_SERVER_STATUS_CONNECT_FAILED
				this.mu.Unlock()
				this.logger.Fatal("server:%s_%s no connection, and connect failed", this.serverName, serverInfo.Addr)
				this.onConnectFailed(serverInfo)
				return nil, err
				//TODO 降低权重
			} else {
				this.mu.Unlock()
				this.logger.Warning("server:%s_%s create new connection failed. err:%s", this.serverName, serverInfo.Addr, err.Error())
			}
		}
	}
	return serverInfo, nil
}

func (this *TClient) onConnShutdown(aConn *TClientConn) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if serverInfo, ok := this.mapServerInfo[aConn.addr]; ok {
		for e := serverInfo.ConnList.Front(); e != nil; e = e.Next() {
			if e.Value == aConn {
				this.logger.Info("remove on connection of addr:%s", aConn.addr)
				serverInfo.ConnList.Remove(e)
				break
			}
		}
	} else {
		this.logger.Warning("not found serverinfo:%s", aConn.addr)
	}
}

func (this *TClient) onConnectFailed(aServerInfo *TServerInfo) {

	validServerNum := 0

	this.mu.RLock()
	for _, serverInfo := range this.mapServerInfo {
		if serverInfo.Status == REMOTE_SERVER_STATUS_NORM || serverInfo.Status == REMOTE_SERVER_STATUS_PAUSE {
			validServerNum++
		}
	}
	this.mu.RUnlock()

	err := fmt.Errorf("server connect failed")
L_for:
	for {
		select {
		case aCall := <-aServerInfo.CallChan:
			gLogger.Debug("failed call:%v", aCall)
			aCall.Error = err
			WhenCallDone(aCall)
		default: //没有了
			break L_for
		}
	}

	this.logger.Warning("server:%s_%s connect failed. validServerNum:%d", this.serverName, aServerInfo.Addr, validServerNum)
	if validServerNum == 0 && this.onClientInvalid != nil {

		this.onClientInvalid(this)
	}

}

func (this *TClient) GetAllServerInfo() []TServerInfoReport {
	this.mu.RLock()
	defer this.mu.RUnlock()
	arr := make([]TServerInfoReport, len(this.mapServerInfo))
	i := 0
	for _, serverInfo := range this.mapServerInfo {
		arr[i].Addr = serverInfo.Addr
		arr[i].Status = serverInfo.Status
		arr[i].Weight = serverInfo.Weight
		arr[i].MaxConnNum = serverInfo.MaxConnNum
		arr[i].CurConnNum = serverInfo.ConnList.Len()
		i++
	}
	return arr
}

func (this *TClient) ChangeServerStatus(aAddr string, aStatus int) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if serverInfo, ok := this.mapServerInfo[aAddr]; ok {
		oldStatus := serverInfo.Status
		serverInfo.Status = aStatus
		this.logger.Info("set server:%s_%s status:%d -> %d", this.serverName, aAddr, oldStatus, aStatus)
		this.updateTotalWeight()
	} else {
		return fmt.Errorf("not found server:%s_%s", this.serverName, aAddr)
	}

	return nil
}

/* vim: set ts=4 sw=4 sts=4 tw=100 noet: */
