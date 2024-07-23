package main

import (
    "auto/autoOrder"
    "fmt"
    "runtime"
    "time"
)

var indexUrl string = "https://vip.stock.finance.sina.com.cn/mkt/"

func main() {
    err := autoOrder.LoadConfig()
    if err != nil {
        fmt.Println(err)
        return
    }
    runtime.GOMAXPROCS(runtime.NumCPU()) // 设置最大逻辑CPU数量

    go autoOrder.MoniterConfigFile()

    for {
        clct := autoOrder.InitSinaQuoteSpider(1)
        // 发起调用
        clct.Visit(indexUrl)

        time.Sleep(700 * time.Millisecond)
    }

    // 集合竞价数据收集、分析
    // autoOrder.GetCallAuctionInfo()

    // 生成监控config
    // SetConfigFile()
}

// 生成监控config
func SetConfigFile() {
    // autoOrder.SetProbemJson()
    autoOrder.SetMoniterConfig()

    fmt.Println("写入 config.json文件完成")
    time.Sleep(3000 * time.Millisecond)
}
