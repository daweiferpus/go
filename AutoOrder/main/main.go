package main

import (
    "auto/autoOrder"
    "fmt"
    "runtime"
    "time"
)

var indexUrl string = "https://vip.stock.finance.sina.com.cn/mkt/"

func main() {

    // 连续竞价，监控、委托
    // AutoOrderMain()

    // 集合竞价数据收集、分析
    // autoOrder.GetCallAuctionInfo()

    // 生成监控config
    // SetConfigFile()

    // 生成excel文件
    autoOrder.SetStockExcel()
}

func AutoOrderMain() {
    err := autoOrder.LoadConfig()
    if err != nil {
        fmt.Println(err)
        return
    }
    runtime.GOMAXPROCS(runtime.NumCPU()) // 设置最大逻辑CPU数量

    go autoOrder.MoniterConfigFile()

Main_Loop:
    for {
        select {
        case <-time.After(700 * time.Millisecond):
            clct := autoOrder.InitSinaQuoteSpider(1)
            // 发起调用
            clct.Visit(indexUrl)
        case <-time.After(6 * time.Hour):
            break Main_Loop
        }
    }
}

// 生成监控config
func SetConfigFile() {
    // autoOrder.SetProbemJson()
    autoOrder.SetMoniterConfig()

    fmt.Println("写入 config.json文件完成")
    time.Sleep(3000 * time.Millisecond)
}
