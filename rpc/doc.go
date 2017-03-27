/*



//测试服务
type TTestService struct {
    counter int64
}

func (this *TTestService) Echo(aMsg string, retMsg *string) error {
    *retMsg = fmt.Sprintf("[%d]%s", atomic.LoadInt64(&this.counter), aMsg)
    atomic.AddInt64(&this.counter, 1)
    return nil
}

//
    addr: 服务监听地址 127.0.0.1:1234
    rpc.NewServerCodec　一个生成编码器接口的函数
    serverConf 服务配置

    服务器配置结构
    type TServerConfig struct {
        ServerIdleTimeout time.Duration //服务器等待请求的超时时间，如果超过这个时间，就会关闭连接
    }
//
rpcServer, err := rpc.NewServer(addr, rpc.NewServerCodec, serverConf)
if err != nil {
    logger.Fatal("new rpc server failed. addr:%s", addr)
    return nil
}

err = rpcServer.RegisterName("test", &TTestService{})
rpcServer.Start()





//
    serviceName: 服务名字
    config: 客户端配置
    rpc.NewClientCodec

    服务器配置结构
    type TClientConfig struct {
        MaxWaitCall    uint32        //一个服务地址上最大等待的请求数
        MaxRetryNum    uint32        //重试次数，＝0表示不重试
        RequestTimeout time.Duration //超时时间，只在同步请求时使用
        PingInterval   time.Duration //ping间隔
    }
//
rpcClient = rpc.NewClient(serviceName, &config, rpc.NewClientCodec)

for _, v := range arrServiceAddr {
    rpcClient.AddServer(v.Ip+":"+v.Port, v.Weight, v.MaxConnNum)
}

err := rpcClient.SyncCall("test.Echo", sendMsg, &retMsg)





*/

package rpc
