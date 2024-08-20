一个简单的监控行情股票数据、自动设置配置文件、生成问题股json文件、写入记录每日涨停股票信息execel的小程序

说明：关于程序中行情数据的分析都是延时行情，不能作为实时行情使用

1、AutoOrderMain 连续竞价行情数据分析，支持行情监控等功能

config.json:

IsMoniter ：// 是否开启监控

{

    Symbol:         // 监控的股票代码
    
    Name:           // 股票名称
    
    WarnPercent:    // 触发监控时的涨幅
    
}

2、GetCallAuctionInfo 集合竞价数据收集、分析，将集合竞价的股票数据分析写入到对应日志文件中

3、SetConfigFile 自动生成监控的config.json文件

4、SetStockExcel 写入记录每日涨停股票信息execel

