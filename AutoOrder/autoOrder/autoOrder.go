package autoOrder

import (
    "fmt"

    "encoding/json"
    "time"

    "github.com/gocolly/colly"
    "github.com/gocolly/colly/extensions"
)

// 入口网址
var indexUrl string = "https://vip.stock.finance.sina.com.cn/mkt/"

// 获取某个市场板块的股票总数
// var countUrl string = "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeStockCount?node=hs_a"

// 第一页
// var quoteFirstPageUrl string = "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=40&sort=symbol&asc=1&node=hs_a&symbol=&_s_r_a=auto"

// 获取涨幅榜前50的股票
var incPrcUrl string = "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=50&sort=changepercent&asc=0&node=hs_a&symbol=&_s_r_a=auto"

// 获取涨幅榜前100的股票
var incPrc100Url string = "https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=100&sort=changepercent&asc=0&node=hs_a&symbol=&_s_r_a=auto"

var sinaClct *colly.Collector

var sinaQuoteDatas []SinaQuoteData

type SinaQuoteData struct {
    Symbol        string  `json:"symbol"`        // 证券内码 含市场
    Code          string  `json:"code"`          // 证券代码
    Name          string  `json:"name"`          // 股票名称
    Trade         string  `json:"trade"`         // 市场价格
    Pricechange   float64 `json:"pricechange"`   // 涨跌价格 涨了几块或跌了几块
    Changepercent float64 `json:"changepercent"` // 涨跌幅
    Buy           string  `json:"buy"`           // 买一价
    Sell          string  `json:"sell"`          // 卖一价
    Settlement    string  `json:"settlement"`    // 昨收盘
    Open          string  `json:"open"`          // 开盘价
    High          string  `json:"high"`          // 最高价
    Low           string  `json:"low"`           // 最低价
    Volume        int     `json:"volume"`        // 成交量  单位 股
    Amount        int     `json:"amount"`        // 成交额  单位 元
    Ticktime      string  `json:"ticktime"`      // 时间，行情时间
    Per           float64 `json:"per"`           // 市盈率
    Pb            float64 `json:"pb"`            // 市净率
    Mktcap        float64 `json:"mktcap"`        // 总市值 单位 万
    Nmc           float64 `json:"nmc"`           // 流通市值 单位 万
    Turnoverratio float64 `json:"turnoverratio"` // 换手率
}

// mode: 1 - 监控行情 , 2 - 集中交易数据分析 3 - config文件设置
// sinaQuoteDatas = make([]SinaQuoteData, 0)
// 使用colly库创建并配置一个网页爬虫。Colly 是一个强大的Go语言网络爬虫框架，用于抓取网页内容
func InitSinaQuoteSpider(mode int) *colly.Collector {
    sinaClct = colly.NewCollector(func(collector *colly.Collector) {
        // colly.AllowURLRevisit()
        // 随机设置用户代理(User-Agent)。用户代理字符串通常被浏览器用来向服务器标识自己，
        // 通过随机化用户代理，可以帮助爬虫避免因使用相同的User-Agent而被网站识别和屏蔽。
        extensions.RandomUserAgent(collector)
        colly.AllowURLRevisit()
    })

    cookiestr := fmt.Sprintf("MONEY-FINANCE-SINA-COM-CN-WEB5=; UOR=,vip.stock.finance.sina.com.cn,; ULV=%d:1:1:1::", time.Now().UnixMilli())

    // 注册一个或多个函数，这些函数会在每个请求发出之前被调用
    // 这对于在发送HTTP请求前修改请求参数、记录日志或执行其他预处理操作非常有用。
    sinaClct.OnRequest(func(r *colly.Request) {
        if r.URL.String() == indexUrl {
            // 设置HTTP请求头中的某个字段值
            r.Headers.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
            r.Headers.Set("accept-encoding", "gzip, deflate, br")
            r.Headers.Set("accept-language", "zh-CN,zh;q=0.9")
            r.Headers.Set("sec-ch-ua", "\"Chromium\";v=\"112\", \"Google Chrome\";v=\"112\", \"Not:A-Brand\";v=\"99\"")
            r.Headers.Set("sec-ch-ua-mobile", "?0")
            r.Headers.Set("sec-ch-ua-platform", "\"Windows\"")
            r.Headers.Set("sec-fetch-dest", "document")
            r.Headers.Set("sec-fetch-mode", "navigate")
            r.Headers.Set("sec-fetch-site", "none")
            r.Headers.Set("sec-fetch-user", "?1")
            r.Headers.Set("upgrade-insecure-requests", "1")
            r.Headers.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
        } else {
            r.Headers.Set("accept", "*/*")
            r.Headers.Set("content-type", "application/x-www-form-urlencoded")
            r.Headers.Set("cookie", cookiestr)
            r.Headers.Set("referer", "https://vip.stock.finance.sina.com.cn/mkt/")
            r.Headers.Set("sec-fetch-dest", "empty")
            r.Headers.Set("sec-fetch-mode", "cors")
            r.Headers.Set("sec-fetch-site", "same-origin")
            r.Headers.Del("sec-fetch-user")
            r.Headers.Del("upgrade-insecure-requests")
        }
    })

    // 注册一个函数，该函数会在每个HTTP响应接收后被调用。
    // 定义一个处理响应的回调函数，该函数会在Colly成功从web服务器接收到响应数据后执行。
    sinaClct.OnResponse(func(r *colly.Response) {
        if r.Request.URL.String() == indexUrl {
            if mode == 2 {
                sinaClct.Visit(incPrc100Url)
            } else {
                sinaClct.Visit(incPrcUrl)
            }

        } else {
            quotes := make([]SinaQuoteData, 0)
            json.Unmarshal(r.Body, &quotes)

            if mode == 1 {
                go MoniterStockDetail(quotes)
            } else if mode == 2 {
                go WriteBidInfo(quotes)
            } else if mode == 3 {
                sinaQuoteDatas = append(sinaQuoteDatas, quotes...)
            }
        }
    })

    // 错误处理
    sinaClct.OnError(func(r *colly.Response, err error) {
        WriteError(fmt.Sprintf("sinaClct.OnResponse err: %s", err))
    })

    return sinaClct
}
