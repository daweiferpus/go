package autoOrder

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "time"

    "github.com/360EntSecGroup-Skylar/excelize"
)

var (
    excelPath = "../config/xuangu/交易日期_2024.xlsx"
    currExcel ExcelOrder // 当前涨停数据
    preExcel  ExcelOrder // 上一日涨停数据

    // 一个月最多五周
    firWeek   = new(OneWeek)
    secWeek   = new(OneWeek)
    thridWeek = new(OneWeek)
    fourWeek  = new(OneWeek)
    fifWeek   = new(OneWeek)

    yesterDay = new(OneDay)

    IsMonFirstDay bool // 一个月的第一天
)

func Init() {
    currExcel.stkInfo = make(map[int][]rowInfo)
    preExcel.stkInfo = make(map[int][]rowInfo)

    yesterDay.MapStk = make(map[int][]OneStk)

    firWeek.Init()
    secWeek.Init()
    thridWeek.Init()
    fourWeek.Init()
    fifWeek.Init()

    IsMonFirstDay = false
}

type rowInfo struct {
    Code          string
    Name          string
    Reason        string
    Money         string
    changepercent float64
}

type ExcelOrder struct {
    day     int
    mon     int
    year    int
    weekNum time.Weekday
    stkInfo map[int][]rowInfo
}

type WeekInfo struct {
    date    string
    weekNum time.Weekday
    stkInfo map[int][]rowInfo
}

func (w *WeekInfo) Init() {
    w.date = ""
    w.weekNum = 0
    w.stkInfo = make(map[int][]rowInfo)
}

func (w *WeekInfo) SetParam(x ...interface{}) {
    switch v := x[0].(type) {
    case string:
        w.date = v
    case time.Weekday:
        w.weekNum = v
    case int:
        if len(x) > 1 {
            value, ok := x[1].(rowInfo)
            if ok {
                w.stkInfo[v] = append(w.stkInfo[v], value)
            }
        }
    }
}

type WeekInfoer interface {
    Init()
    SetParam(x ...interface{})
}

type WeekTotal struct {
    MaxNum      []int      // 每个板的最大板数
    statekTotal []WeekInfo // 每天的信息
}

type WriteExceler interface {
    Init()
    ModifyExcel(v interface{})
}

func (wk *WeekTotal) Init() {
    wk.MaxNum = make([]int, 0)
    wk.statekTotal = make([]WeekInfo, 0)
}

func (wk *WeekTotal) ModifyExcel(v interface{}) {
    switch value := v.(type) {
    case int:
        wk.MaxNum = append(wk.MaxNum, value)
    case WeekInfo:
        wk.statekTotal = append(wk.statekTotal, value)
    default:
        break
    }
}

func convertChineseNumToDecimal(inputStr string) string {
    // 定义单位和对应的数值
    units := map[string]float64{"万": 10000, "亿": 100000000}

    // 使用正则表达式去除中文逗号，并获取数字和单位
    re := regexp.MustCompile(`(\d+,?\d*\.\d*)\s*([万亿]?)`)
    matches := re.FindStringSubmatch(inputStr)

    if len(matches) < 3 {
        return ""
    }

    numStr := strings.Replace(matches[1], ",", "", -1)
    unit := matches[2]

    // 将数字字符串转换为浮点数
    num, err := strconv.ParseFloat(numStr, 64)
    if err != nil {
        return ""
    }

    // 根据单位进行转换
    if unit == "" {
        num *= units["万"]
    } else {
        num *= units[unit]
    }

    // 格式化输出
    return fmt.Sprintf("%.2f亿", num/100000000)
    // return fmt.Sprintf("%.2f万", num/10000)
}

func (e *ExcelOrder) GetOrderInfo(readSheet [][]string) {
    if e.stkInfo == nil {
        e.stkInfo = make(map[int][]rowInfo)
    }
    for rowNum, row := range readSheet {
        indexKey := 0
        var stRowInfo rowInfo
        for sheetNum, sheeCol := range row {
            if rowNum == 56 && sheetNum == 0 {
                // 涨停 2024.07.31
                dateStr := sheeCol[strings.Index(sheeCol, "2"):]
                // 指定布局字符串，与dateStr的格式相匹配
                layout := "2006.01.02"
                // 使用Parse函数解析日期字符串
                t, err := time.Parse(layout, dateStr)
                if err != nil {
                    fmt.Println("Error parsing date:", err)
                    return
                }
                e.weekNum = t.Weekday()
                // 2006.01.02
                arrTemp := strings.Split(dateStr, ".")
                if len(arrTemp) > 2 {
                    e.year, _ = strconv.Atoi(arrTemp[0])
                    e.mon, _ = strconv.Atoi(arrTemp[1])
                    e.day, _ = strconv.Atoi(arrTemp[2])
                }
            }
            if rowNum > 56 {
                switch sheetNum {
                case 2:
                    stRowInfo.Code = strings.TrimSpace(sheeCol)
                case 3:
                    stRowInfo.Name = strings.TrimSpace(sheeCol)
                case 5:
                    stRowInfo.changepercent, _ = strconv.ParseFloat(strings.TrimSpace(sheeCol), 64)
                case 9:
                    indexKey, _ = strconv.Atoi(strings.TrimSpace(sheeCol))
                case 10:
                    stRowInfo.Reason = strings.TrimSpace(sheeCol)
                case 12:
                    // 2,096.27万
                    stRowInfo.Money = convertChineseNumToDecimal(strings.TrimSpace(sheeCol))
                default:
                    break
                }

            }
        }
        if indexKey > 0 {
            e.stkInfo[indexKey] = append(e.stkInfo[indexKey], stRowInfo)
        }
    }
}

func GetExcelInfo(filepath, sheetName string) [][]string {
    f, err := excelize.OpenFile(filepath)
    if err != nil {
        fmt.Println(err)
        return nil
    }
    // 检查工作表是否存在，存在就删除
    i := f.GetSheetIndex(sheetName)
    if i > 0 {
        rows := f.GetRows(sheetName)
        if len(rows) > 0 {
            return rows
        }
        return nil
    }

    // 根据给定的工作表名称和单元格引用从单元格中获取值。
    // cell := f.GetCellValue("Sheet1", "B2")
    // 获取Sheet1中的所有行。

    return nil
}

func SetStockExcel() {
    // 获取当日数据
    readSheet := GetExcelInfo(excelPath, "Sheet1")
    if readSheet == nil {
        fmt.Println("Sheet1 is nil")
        return
    }
    // 将sheet1的格式转为ExcelOrder
    currExcel.GetOrderInfo(readSheet)

    // 获取前日数据
    readSheet = GetExcelInfo(excelPath, "Sheet2")
    if readSheet == nil {
        fmt.Println("Sheet2 is nil")
        return
    }
    // 将sheet2的格式转为ExcelOrder
    preExcel.GetOrderInfo(readSheet)

    if currExcel.mon <= 0 {
        fmt.Println("写入失败...")
        return
    }
    sheetName := ""
    // 修改excel, 确认修改的是第一个月的数据
    if currExcel.weekNum != time.Monday && currExcel.day < 7 && currExcel.mon != 10 {
        sheetName = fmt.Sprintf("%d月", currExcel.mon-1)
    } else {
        sheetName = fmt.Sprintf("%d月", currExcel.mon)
    }

    fmt.Printf("正在写入表格: %s ...\n", sheetName)
    // 获取写入单元格
    writeSheet := GetExcelInfo(excelPath, sheetName)
    if writeSheet == nil { // 一个月的第一天，取sheet取不到，就取上一个月的
        IsMonFirstDay = true
        lastMon := fmt.Sprintf("%d月", currExcel.mon-1)
        writeSheet := GetExcelInfo(excelPath, lastMon)
        if writeSheet == nil {
            fmt.Println("write sheet is nil")
            return
        }
    }

    // 将excel的二维数组转为OneWeek结构体数据
    GetWriteSheet(writeSheet, sheetName)

    WriteExcel(sheetName)

    fmt.Println("写入成功")
}
