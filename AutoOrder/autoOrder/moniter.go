package autoOrder

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "math"
    "net/http"
    "os"
    "path"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/fsnotify/fsnotify"
    "golang.org/x/exp/slices"

    "golang.org/x/text/encoding/simplifiedchinese"
    "golang.org/x/text/transform"
)

type Config struct {
    OrderInfo  OrderInfo     `json:"OrderInfo"`
    ArrMoniter []MoniterInfo `json:"Source"`
}

type MoniterInfo struct {
    Symbol        string  `json:"Symbol"`
    Changepercent float64 `json:"WarnPercent"`
    IsMoniter     bool    `json:"IsMonitor"` // 注意这里使用了json作为键，并且遵循了正确的格式
}

type OrderInfo struct {
    IsFirstOrder  bool    `json:"IsFirstOrder"`  // 打首板委托开关
    FirstLimitNum int     `json:"FirstLimitNum"` // 首板数量 超过该数量之后的板不打
    IsSecondOrder bool    `json:"IsSecondOrder"` // 打二板委托开关
    IsFirst       bool    `json:"IsFirst"`       // 第一个上板的是否委托
    OrderMoney    float64 `json:"OrderMoney"`    // 委托金额
}

type StockPlm struct {
    Code string
    Name string
}

type ProblemStock struct {
    ArrStock []StockPlm
}

var (
    stkLog     *Logger
    moniterLog *Logger
    errorLog   *Logger
    bidLog     *Logger

    bidMoniter *Logger

    svrConfig      Config
    configFilePath = "../config/config.json"

    // 问题股信息
    problemStock ProblemStock
    // 重启同花顺客户端请求
    thsStartUrl = "http://127.0.0.1:5000/thsauto/client/restart"
    // 买入下单请求
    thsOrderUrl = "http://127.0.0.1:5000/thsauto/buy?stock_no="
    mapSinadata map[string][]SinaQuoteData

    arrOrders []string // 已经委托的股票集合

    firstLimitNum int // 首板数量

    updateConfigChan = make(chan *Config) // 用来实时更新config配置文件的
)

// 浮点数比较 小于或等于
func lessOrEqual(a, b, epsilon float64) bool {
    return a < b || math.Abs(a-b) <= epsilon
}

func GetCurrentDirectory() (string, error) {
    dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
    if err != nil {
        return "", err
    }
    return strings.Replace(dir, "\\", "/", -1), nil
}

func init() {
    ret, err := GetCurrentDirectory()
    if err != nil {
        fmt.Println(err)
    }
    configFilePath = path.Join(ret, configFilePath)

    now := time.Now()

    StkFileName := fmt.Sprintf("../log/stockInfo/stock_%04d_%02d_%02d.log", now.Year(), now.Month(), now.Day())
    stkLog, err = NewFileLogger(StkFileName)
    if err != nil {
        fmt.Println(err)
    }

    MoniterFileName := fmt.Sprintf("../log/moniter/moniter_%04d_%02d_%02d.log", now.Year(), now.Month(), now.Day())
    moniterLog, err = NewFileLogger(MoniterFileName)
    if err != nil {
        fmt.Println(err)
    }

    bidFileName := fmt.Sprintf("../log/bidInfo/bid_%04d_%02d_%02d.log", now.Year(), now.Month(), now.Day())
    bidLog, err = NewFileLogger(bidFileName)
    if err != nil {
        fmt.Println(err)
    }

    bidMoniterFile := fmt.Sprintf("../log/bidInfo/bid_moniter_%04d_%02d_%02d.log", now.Year(), now.Month(), now.Day())
    bidMoniter, err = NewFileLogger(bidMoniterFile)
    if err != nil {
        fmt.Println(err)
    }

    errorLog, err = NewFileLogger("../log/error.log")
    if err != nil {
        fmt.Println(err)
    }

    mapSinadata = make(map[string][]SinaQuoteData, 0)

    arrOrders = make([]string, 0)

    path, err := os.Executable()
    if err != nil {
        fmt.Println("Error getting executable path:", err)
        return
    }

    filePath := ""
    if strings.Contains(path, "config") {
        filePath = "problem.json"
    } else {
        filePath = "../config/problem.json"
    }
    conFile, err := ioutil.ReadFile(filePath)
    if err != nil {
        fmt.Println(err)
        return
    }

    err = json.Unmarshal(conFile, &problemStock)
    if err != nil {
        fmt.Println(err)
        return
    }

    firstLimitNum = 0
}

func LoadConfig() error {
    conFile, err := ioutil.ReadFile(configFilePath)
    if err != nil {
        fmt.Println(err)
        return err
    }

    err = json.Unmarshal(conFile, &svrConfig)
    if err != nil {
        fmt.Println(err)
        return err
    }
    return nil
}

func MoniterConfigFile() {
    go watchConfigFile(configFilePath, updateConfigChan)

    for {
        time.Sleep(1 * time.Second)
        select {
        case config := <-updateConfigChan:
            svrConfig = *config
            fmt.Println("update config success : ", svrConfig.OrderInfo.FirstLimitNum)
        }
    }
}

func GetConfig() *Config {
    return &svrConfig
}

type Response struct {
    Code      int         `json:"code"`
    EntrustNo string      `json:"entrust_no"`
    Msg       string      `json:"msg"`
    Status    interface{} `json:"status"` // 使用interface{}来处理任何类型
}

// 发送http请求
func GetHttps(url string, wg *sync.WaitGroup) bool {
    defer func() {
        if wg != nil {
            wg.Done()
        }
    }()
    // Create a new request using http.NewRequest
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        WriteError(fmt.Sprintf("Error creating request: %s", err))
        return false
    }

    // Make the request with a custom HTTP client (to control timeouts, etc.)
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        WriteError(fmt.Sprintf("Error sending request: %s", err))
        return false
    }
    defer resp.Body.Close()

    if !strings.Contains(url, "restart") {
        // Process the response as before
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            WriteError(fmt.Sprintf("Error reading response body: %s", err))
            return false
        }
        var response Response
        err = json.Unmarshal(body, &response)
        if err != nil {
            WriteError(fmt.Sprintf("%s get fail : %s", url, response.Msg))
            return false
        }
        moniterLog.WriteInfo(fmt.Sprintf("%s : %s", url, response.Msg))
        return response.Code == 0
    }
    return true
}

func WriteHqInfo(data SinaQuoteData, wg *sync.WaitGroup) {
    defer wg.Done()
    strLog := fmt.Sprintf("symbol:%s,Code:%s,name:%s,trade:%s,pricechange:%.3f,changepercent:%.3f,buy:%s,sell:%s,"+
        "settlement:%s,open:%s,high:%s,low:%s,volume:%d,amount:%d,ticktime:%s,per:%.3f,pb:%.3f,mktcap:%.3f,"+
        "nmc:%.3f,turnoverratio:%.3f", data.Symbol, data.Code, data.Name, data.Trade, data.Pricechange, data.Changepercent,
        data.Buy, data.Sell, data.Settlement, data.Open, data.High, data.Low, data.Volume, data.Amount, data.Ticktime,
        data.Per, data.Pb, data.Mktcap, data.Nmc, data.Turnoverratio)

    if stkLog != nil {
        stkLog.WriteInfo(strLog)
    }
}

func WriteBidInfo(sinaQuoteDatas []SinaQuoteData) {
    for _, data := range sinaQuoteDatas {
        strLog := fmt.Sprintf("symbol:%s,Code:%s,name:%s,trade:%s,pricechange:%.3f,changepercent:%.3f,buy:%s,sell:%s,"+
            "settlement:%s,open:%s,high:%s,low:%s,volume:%d,amount:%d,ticktime:%s,per:%.3f,pb:%.3f,mktcap:%.3f,"+
            "nmc:%.3f,turnoverratio:%.3f", data.Symbol, data.Code, data.Name, data.Trade, data.Pricechange, data.Changepercent,
            data.Buy, data.Sell, data.Settlement, data.Open, data.High, data.Low, data.Volume, data.Amount, data.Ticktime,
            data.Per, data.Pb, data.Mktcap, data.Nmc, data.Turnoverratio)
        if bidLog != nil {
            bidLog.WriteInfo(strLog)
        }
    }
}

// 记录监控触发后的信息
func WriteMoniterInfo(data SinaQuoteData, wg *sync.WaitGroup) {
    defer wg.Done()
    // 获取当前时间
    currentTime := time.Now()
    strLog := fmt.Sprintf("启动同花顺成功,监控代码: %s, 股票名称:%s, 当前涨幅:%.2f, 当前价格: %s",
        data.Code, data.Name, data.Changepercent, data.Trade)

    fmt.Println("当前时间:", currentTime.Format("15:04:05"), " 行情时间:", data.Ticktime, strLog)
    if moniterLog != nil {
        moniterLog.WriteInfo(strLog)
    }
}

// 记录委托触发后的信息
func WriteOrderInfo(data SinaQuoteData, wg *sync.WaitGroup) {
    wg.Done()
    // 获取当前时间
    currentTime := time.Now()
    strLog := fmt.Sprintf("监控下单成功,委托代码: %s, 股票名称:%s, 当前涨幅:%.2f, 委托价格: %s,行情时间:%s",
        data.Code, data.Name, data.Changepercent, data.Trade, data.Ticktime)

    fmt.Println("当前时间:", currentTime.Format("15:04:05"), " 行情时间:", data.Ticktime, strLog)
    if moniterLog != nil {
        moniterLog.WriteInfo(strLog)
    }
}

// 记录监控错误信息
func WriteError(str string) {
    if errorLog != nil {
        errorLog.WriteInfo(str)
    }
}

func SetMoniterConfig() {
    clct := InitSinaQuoteSpider(3)
    // 发起调用
    clct.Visit(indexUrl)

    time.Sleep(1000 * time.Millisecond)

    //arrTmp := []MoniterInfo{}
    // var arrTTmp = make([]MoniterInfo, 0)
    //var arrTTmp []MoniterInfo = make([]MoniterInfo, 0)

    svrConfig.OrderInfo.IsSecondOrder = false
    svrConfig.OrderInfo.IsFirstOrder = false
    svrConfig.OrderInfo.IsFirst = false
    svrConfig.OrderInfo.FirstLimitNum = 1
    svrConfig.OrderInfo.OrderMoney = 11000.00

    if len(sinaQuoteDatas) > 0 {
        for _, v := range sinaQuoteDatas {
            if strings.Contains(strings.ToLower(v.Symbol), "sz") && !strings.Contains(strings.ToLower(v.Name), "st") { // 深圳 非ST
                if strings.HasPrefix(strings.ToLower(v.Name), "n") || strings.HasPrefix(strings.ToLower(v.Name), "c") {
                    continue
                }
                if strings.Compare(v.Trade, v.High) == 0 && strings.Compare(v.Sell, "0.000") == 0 &&
                    !lessOrEqual(v.Changepercent, 15.00, 0.00001) { // 20cm 涨停
                    // 过滤问题股
                    bProblem := false
                    for _, pro := range problemStock.ArrStock {
                        if strings.Compare(pro.Code, v.Symbol) == 0 {
                            fmt.Printf("Code : %s, Name : %s 是问题股\n", pro.Code, pro.Name)
                            bProblem = true
                            break
                        }
                    }
                    if bProblem {
                        continue
                    }
                    dTrade, _ := strconv.ParseFloat(v.Trade, 64)
                    // 市价小于6块 且 (市值 < 30 || 市盈率 < 0)
                    if lessOrEqual(dTrade, 6.00, 0.000001) && (lessOrEqual(v.Mktcap, 300000.0, 0.00001) ||
                        lessOrEqual(v.Per, 0.00, 0.00001)) {
                        continue
                    } else if !lessOrEqual(dTrade, 3.00, 0.000001) {
                        svrConfig.ArrMoniter = append(svrConfig.ArrMoniter, MoniterInfo{Symbol: v.Symbol, Changepercent: 15.00, IsMoniter: false})
                    }

                }
            }
        }
    }
    if len(svrConfig.ArrMoniter) == 0 {
        fmt.Print("没有符合条件的创业板")
        svrConfig.ArrMoniter = append(svrConfig.ArrMoniter, MoniterInfo{Symbol: "sz300750", Changepercent: 15.00, IsMoniter: false})
    }

    // 使用json.MarshalIndent美化输出的JSON（可选，增加可读性）
    jsonData, err := json.MarshalIndent(svrConfig, "", "  ")
    if err != nil {
        fmt.Println("Error marshaling to JSON:", err)
        return
    }

    // 打开一个文件用于写入
    file, err := os.Create("config.json")
    if err != nil {
        fmt.Println("Error creating file:", err)
        return
    }
    defer file.Close()

    // 写入JSON数据到文件
    _, err = file.Write(jsonData)
    if err != nil {
        fmt.Println("Error writing to file:", err)
        return
    }
}

func SetProbemJson() {
    // 打开文件
    file, err := os.Open("问题股.txt")
    if err != nil {
        fmt.Println("open file err:", err.Error())
        return
    }

    // 处理结束后关闭文件
    defer file.Close()
    // 使用bufio读取
    r := bufio.NewReader(file)

    var problemStock ProblemStock
    for {
        // 分行读取文件  ReadLine返回单个行，不包括行尾字节(\n  或 \r\n)
        filedata, _, err := r.ReadLine()

        // 读取到末尾退出
        if err == io.EOF {
            break
        }

        if err != nil {
            fmt.Println("read err", err.Error())
            break
        }

        data := string(filedata)
        data = strings.TrimSpace(data)

        if len(data) > 0 && data[0] >= 65 && data[0] <= 122 {
            arrTmp := strings.Split(data, "\t")

            if len(arrTmp) > 1 {
                gbkBytes := []byte(arrTmp[1]) // 这些字节代表GBK编码
                // 创建一个GBK的解码器
                decoder := simplifiedchinese.GBK.NewDecoder()
                // 使用解码器将GBK字节转换为UTF-8
                utf8Bytes, _, err := transform.Bytes(decoder, gbkBytes)
                if err != nil {
                    fmt.Println("Error decoding:", err)
                    return
                }

                // utf8Bytes现在包含的是UTF-8编码的字节
                utf8String := string(utf8Bytes)

                problemStock.ArrStock = append(problemStock.ArrStock, StockPlm{strings.ToLower(arrTmp[0]), utf8String})
            }

        }
    }
    // 使用json.MarshalIndent美化输出的JSON（可选，增加可读性）
    jsonData, err := json.MarshalIndent(problemStock, "", "  ")
    if err != nil {
        fmt.Println("Error marshaling to JSON:", err)
        return
    }

    // 打开一个文件用于写入
    fileW, errW := os.Create("problem.json")
    if errW != nil {
        fmt.Println("Error creating file:", errW)
        return
    }
    defer fileW.Close()

    // 写入JSON数据到文件
    _, errW = fileW.Write(jsonData)
    if errW != nil {
        fmt.Println("Error writing to file:", errW)
        return
    }

}

func MoniterFirstLimitUp(v SinaQuoteData, wg *sync.WaitGroup) {
    defer wg.Done()

    var wgThis sync.WaitGroup
    // 昨收盘
    price_sett, _ := strconv.ParseFloat(v.Settlement, 64)
    //涨停价
    price_max := math.Round(price_sett*120) / 100 // 四舍五入到两位小数
    // 市场价
    price_trade, _ := strconv.ParseFloat(v.Trade, 64)

    // 开盘价
    openPrice, _ := strconv.ParseFloat(v.Open, 64)

    if math.Abs(price_max-price_trade) <= 0.0001 && strings.Compare(v.Sell, "0.000") == 0 {
        arrOrders = append(arrOrders, v.Symbol)

        // 排除问题股
        for _, pro := range problemStock.ArrStock {
            if strings.Compare(pro.Code, v.Symbol) == 0 {
                return
            }
        }
        if lessOrEqual(price_trade, 3.00, 0.000001) {
            return
        }
        // 市价小于6块 且 (市值 < 30 || 市盈率 < 0)
        if lessOrEqual(price_trade, 6.00, 0.000001) && (lessOrEqual(v.Mktcap, 300000.0, 0.00001) ||
            lessOrEqual(v.Per, 0.00, 0.00001)) {
            return
        }

        // 获取当前时间
        now := time.Now()
        // 创建一个今天的9点40分的时间点
        today940 := time.Date(now.Year(), now.Month(), now.Day(), 9, 40, 0, 0, now.Location())
        // 开盘价大于15% 且 在9点40之前
        if lessOrEqual(price_sett*1.15, openPrice, 0.00001) && now.Before(today940) {
            return
        }

        // http://127.0.0.1:5000/thsauto/buy?stock_no=301300&price=30.84&amount=300
        money := svrConfig.OrderInfo.OrderMoney
        num := int(money/price_max/100) * 100
        url := fmt.Sprintf("%s%s&price=%.2f&amount=%d", thsOrderUrl, v.Code, price_max, num)
        firstLimitNum++
        wgThis.Add(2)
        go func() {
            defer wgThis.Done()
            for i := 0; i < 3; i++ { // 失败重发3次
                if GetHttps(url, nil) {
                    break
                }
                time.Sleep(500 * time.Millisecond)
            }
        }()

        moniterLog.WriteInfo(url)
        go WriteOrderInfo(v, &wgThis)
    }

    wgThis.Wait()
}

func MoniterStockOrder(v SinaQuoteData, wg *sync.WaitGroup) {
    defer wg.Done()

    var wgThis sync.WaitGroup

    // 昨收盘
    price_sett, _ := strconv.ParseFloat(v.Settlement, 64)
    //涨停价
    price_max := math.Round(price_sett*120) / 100 // 四舍五入到两位小数
    // 市场价
    price_trade, _ := strconv.ParseFloat(v.Trade, 64)

    // 开盘价
    openPrice, _ := strconv.ParseFloat(v.Open, 64)
    // 获取当前时间
    now := time.Now()
    // 创建一个今天的9点40分的时间点
    today940 := time.Date(now.Year(), now.Month(), now.Day(), 9, 40, 0, 0, now.Location())

    if math.Abs(price_max-price_trade) <= 0.0001 && strings.Compare(v.Sell, "0.000") == 0 {
        // 排除问题股
        // for _, pro := range problemStock.ArrStock {
        // 	if strings.Compare(pro.Code, v.Symbol) == 0 {
        // 		return
        // 	}
        // }

        arrOrders = append(arrOrders, v.Symbol)

        // 过滤掉第一支上板的股票
        if !svrConfig.OrderInfo.IsFirst {
            svrConfig.OrderInfo.IsFirst = true
            return
        }

        // 开盘价大于15% 且 在9点40之前
        if lessOrEqual(price_sett*1.15, openPrice, 0.00001) && now.Before(today940) {
            return
        }

        // http://127.0.0.1:5000/thsauto/buy?stock_no=301300&price=30.84&amount=300
        money := svrConfig.OrderInfo.OrderMoney
        num := int(money/price_max/100) * 100
        url := fmt.Sprintf("%s%s&price=%.2f&amount=%d", thsOrderUrl, v.Code, price_max, num)
        wgThis.Add(2)
        go func() {
            defer wgThis.Done()
            for i := 0; i < 3; i++ { // 失败重发3次
                if GetHttps(url, nil) {
                    break
                }
                time.Sleep(500 * time.Millisecond)
            }
        }()

        moniterLog.WriteInfo(url)
        go WriteOrderInfo(v, &wgThis)
    }
    wgThis.Wait()
}

func MoniterStockDetail(sinaQuoteDatas []SinaQuoteData) {
    var wg sync.WaitGroup
    wg.Add(2)
    go func() {
        defer wg.Done()
        var wgChild sync.WaitGroup
        for i, moniter := range svrConfig.ArrMoniter {
            for _, v := range sinaQuoteDatas {
                if strings.EqualFold(v.Symbol, moniter.Symbol) {
                    // 保存匹配到的行情数据
                    wgChild.Add(1)
                    go WriteHqInfo(v, &wgChild)

                    // 监控
                    if !moniter.IsMoniter && !lessOrEqual(v.Changepercent, moniter.Changepercent, 0.000001) {
                        // 重启同花顺客户端
                        wgChild.Add(2)
                        go GetHttps(thsStartUrl, &wgChild)
                        go WriteMoniterInfo(v, &wgChild)
                        svrConfig.ArrMoniter[i].IsMoniter = true
                    }

                    // 委托
                    if svrConfig.OrderInfo.IsSecondOrder && !slices.Contains(arrOrders, v.Symbol) {
                        wgChild.Add(1)
                        go MoniterStockOrder(v, &wgChild)
                    }
                    break
                }
            }
        }
        wgChild.Wait()
    }()

    go func() {
        defer wg.Done()
        var wgChild sync.WaitGroup
        for _, v := range sinaQuoteDatas {
            if svrConfig.OrderInfo.IsFirstOrder && svrConfig.OrderInfo.FirstLimitNum > firstLimitNum &&
                strings.HasPrefix(v.Symbol, "sz") && !slices.Contains(arrOrders, v.Symbol) &&
                !strings.Contains(strings.ToLower(v.Name), "st") {
                wgChild.Add(1)
                go MoniterFirstLimitUp(v, &wgChild)
            }
        }
        wgChild.Wait()
    }()

    wg.Wait()
}

func GetCallAuctionInfo() {
    for {
        // 获取当前时间
        now := time.Now()
        // 创建一个代表9:20:00的时间对象
        startTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 20, 0, 0, now.Location())
        // 创建一个代表9:25:15的时间对象 因为延时的问题延后15s
        endTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 25, 15, 0, now.Location())

        if now.After(startTime) && now.Before(endTime) {
            clct := InitSinaQuoteSpider(2)
            // 发起调用
            clct.Visit(indexUrl)
            time.Sleep(500 * time.Millisecond)
        } else if now.After(endTime) {
            break
        }
    }

    time.Sleep(1000 * time.Millisecond)
    fmt.Println("集合竞价信息记录完成")
    fmt.Println("开始分析集合竞价信息")
    time.Sleep(2000 * time.Millisecond)

    CalcBidLogger()
    MoniterBidInfo()
}

func CalcBidLogger() {
    // 打开日志文件
    now := time.Now()
    bidFileName := fmt.Sprintf("../log/bidInfo/bid_%04d_%02d_%02d.log", now.Year(), now.Month(), now.Day())
    logFile, err := os.Open(bidFileName)
    if err != nil {
        log.Fatalf("Error opening file: %v", err)
    }
    defer logFile.Close()

    // 创建一个缓冲扫描器来读取文件
    scanner := bufio.NewScanner(logFile)

    // 逐行读取文件
    for scanner.Scan() {
        var sa SinaQuoteData
        line := scanner.Text()

        line = strings.TrimSpace(line)

        pos := strings.Index(line, "symbol:")
        strSymbol := ""
        if pos > -1 {
            line = line[pos+7:]
            strSymbol = line[:strings.Index(line, ",")]
            sa.Symbol = strSymbol
        }

        pos = strings.Index(line, "Code:")
        if pos > -1 {
            line = line[pos+5:]
            sa.Code = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "name:")
        if pos > -1 {
            line = line[pos+5:]
            sa.Name = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "trade:")
        if pos > -1 {
            line = line[pos+6:]
            sa.Trade = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "pricechange:")
        if pos > -1 {
            line = line[pos+12:]
            sa.Pricechange, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "changepercent:")
        if pos > -1 {
            line = line[pos+14:]
            sa.Changepercent, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "buy:")
        if pos > -1 {
            line = line[pos+4:]
            sa.Buy = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "sell:")
        if pos > -1 {
            line = line[pos+5:]
            sa.Sell = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "settlement:")
        if pos > -1 {
            line = line[pos+11:]
            sa.Settlement = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "open:")
        if pos > -1 {
            line = line[pos+5:]
            sa.Open = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "high:")
        if pos > -1 {
            line = line[pos+5:]
            sa.High = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "low:")
        if pos > -1 {
            line = line[pos+4:]
            sa.Low = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "volume:")
        if pos > -1 {
            line = line[pos+7:]
            intTmp, _ := strconv.ParseInt(line[:strings.Index(line, ",")], 10, 64)
            sa.Volume = int(intTmp)
        }

        pos = strings.Index(line, "amount:")
        if pos > -1 {
            line = line[pos+7:]
            intTmp, _ := strconv.ParseInt(line[:strings.Index(line, ",")], 10, 64)
            sa.Amount = int(intTmp)
        }

        pos = strings.Index(line, "ticktime:")
        if pos > -1 {
            line = line[pos+9:]
            sa.Ticktime = line[:strings.Index(line, ",")]
        }

        pos = strings.Index(line, "per:")
        if pos > -1 {
            line = line[pos+4:]
            sa.Per, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "pb:")
        if pos > -1 {
            line = line[pos+3:]
            sa.Pb, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "mktcap:")
        if pos > -1 {
            line = line[pos+7:]
            sa.Mktcap, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "nmc:")
        if pos > -1 {
            line = line[pos+4:]
            sa.Nmc, _ = strconv.ParseFloat(line[:strings.Index(line, ",")], 64)
        }

        pos = strings.Index(line, "turnoverratio:")
        if pos > -1 {
            line = line[pos+14:]

            sa.Turnoverratio, _ = strconv.ParseFloat(line, 64)
        }

        _, ok := mapSinadata[sa.Symbol]
        if ok {
            mapSinadata[sa.Symbol] = append(mapSinadata[sa.Symbol], sa)
        } else {
            slice := make([]SinaQuoteData, 0)
            slice = append(slice, sa)
            mapSinadata[sa.Symbol] = slice
        }
    }

    if err := scanner.Err(); err != nil {
        log.Fatalf("Error reading file: %v", err)
    }
}

type SinaQuoteDatas []SinaQuoteData

func (p SinaQuoteDatas) Len() int {
    return len(p)
}

func (p SinaQuoteDatas) Less(i, j int) bool {
    if strings.Compare(p[i].Ticktime, p[j].Ticktime) < 0 {
        return true
    } else {
        return false
    }
}

func (p SinaQuoteDatas) Swap(i, j int) {
    p[i], p[j] = p[j], p[i]
}

func MoniterBidInfo() {
    for _, v := range mapSinadata {
        // 排序
        sort.Sort(SinaQuoteDatas(v))

        var lastSina SinaQuoteData
        var sliceMoniter = make([]string, 0)
        var sliceMoniterTmp = make([]string, 0)
        lastTime, _ := time.Parse("15:04:05", "08:30:00")
        for _, sinInfo := range v {
            hqTime, _ := time.Parse("15:04:05", sinInfo.Ticktime)
            duration := hqTime.Sub(lastTime)

            if duration <= 2*time.Second && duration != 0 {
                if sinInfo.Changepercent-lastSina.Changepercent-3.00 > 0.00001 {
                    if !slices.Contains(sliceMoniter, sinInfo.Code) {
                        strTmp := fmt.Sprintf("两秒内涨幅波动超过3%%:%s, Name:%s, Time:%s", sinInfo.Code,
                            sinInfo.Name, sinInfo.Ticktime)
                        bidMoniter.WriteInfo(strTmp)
                        sliceMoniter = append(sliceMoniter, sinInfo.Code)
                    }
                }
            }

            if strings.Compare(sinInfo.Ticktime, "09:25:00") == 0 {
                if sinInfo.Nmc-400000.00 <= 0.00001 && sinInfo.Amount-30000000 > 0 {
                    if !slices.Contains(sliceMoniterTmp, sinInfo.Code) {
                        strTmp := fmt.Sprintf("集合竞价成交超3000万:%s,Name:%s, 成交量:%d万, 开盘价:%s, 开盘涨幅:%.2f", sinInfo.Code,
                            sinInfo.Name, sinInfo.Amount/10000, sinInfo.Open, sinInfo.Changepercent)
                        bidMoniter.WriteInfo(strTmp)
                        sliceMoniterTmp = append(sliceMoniterTmp, sinInfo.Code)
                    }
                }
            }
            lastTime = hqTime
            lastSina = sinInfo
        }
    }
}

func watchConfigFile(configPath string, updateChan chan<- *Config) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Fatal(err)
    }
    defer watcher.Close()

    done := make(chan bool)

    go func() {
        for {
            time.Sleep(1 * time.Second)
            select {
            case event, ok := <-watcher.Events:
                if !ok {
                    return
                }
                if event.Op&fsnotify.Write == fsnotify.Write {
                    err := LoadConfig()
                    if err != nil {
                        log.Println("Error loading config:", err)
                        continue
                    }
                    updateChan <- &svrConfig
                }
            case err, ok := <-watcher.Errors:
                if !ok {
                    return
                }
                log.Println("Error:", err)
            }
        }
    }()

    err = watcher.Add(configPath)
    if err != nil {
        log.Fatal(err)
    }

    <-done
}
