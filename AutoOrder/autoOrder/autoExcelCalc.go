package autoOrder

import (
    "fmt"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/xuri/excelize/v2"
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

// 今日涨停和昨日涨停数据
// sheet1和sheet2里的一行数据
type rowInfo struct {
    Code          string
    Name          string
    Reason        string
    Money         string
    changepercent float64
}

// 今日涨停和昨日涨停数据
type ExcelOrder struct {
    day     int
    mon     int
    year    int
    weekNum time.Weekday
    stkInfo map[int][]rowInfo
}

// 一只股票的信息
type OneStk struct {
    Code    string
    Name    string
    Reason  string
    Money   string
    Perform string
    PreRate string
}

// 一天的股票信息 map[int][]OneStk, int:表示第几板 []OneStk股票信息集合
type OneDay struct {
    Date    string
    WeekNum time.Weekday
    MapStk  map[int][]OneStk // 格式为 arrStk[1].[0] 连板数为1的第一支股票
}

// 一周的股票信息  Maxboard 表示每个连板占excel的行数数组，例如 [4 4 4 32] 分别代表 4,3,2,1连板的行数
// []OneDay每天股票信息的集合
type OneWeek struct {
    Maxboard []int // 最大的板块数
    ArrDay   []OneDay
}

// 接口用来操作切换每周的数据，最多只配置了五周，一个月的
type Weeker interface {
    Init()
    SetParam(x interface{})
    PushStk(int, int, OneStk)
    GetMaxBoard() []int
    GetArrDay() []OneDay
}

// func (w *OneWeek) Show() {
//     fmt.Printf("ArrDay : %p\n", w.ArrDay)
//     fmt.Printf("Maxboard : %p\n", w.Maxboard)
// }

// 初始化
func (w *OneWeek) Init() {
    w.Maxboard = make([]int, 0)
    w.ArrDay = make([]OneDay, 0)
}

// 插入连板格子数量
func (w *OneWeek) SetParam(x interface{}) {
    switch v := x.(type) {
    case int:
        w.Maxboard = append(w.Maxboard, v)
    case []int:
        w.Maxboard = v
    case OneDay:
        w.ArrDay = append(w.ArrDay, v)
    }
}

// 写入一天的股票数据 day:第几天 k:第几板
func (w *OneWeek) PushStk(day, k int, oneStk OneStk) {
    if len(w.ArrDay) < day+1 {
        fmt.Println(day, k, oneStk, "写入失败")
        return
    }
    w.ArrDay[day].MapStk[k] = append(w.ArrDay[day].MapStk[k], oneStk)
}

// 获取每个连板占excel的行数数组，例如 [4 4 4 32] 分别代表 4,3,2,1连板的行数
func (w *OneWeek) GetMaxBoard() []int {
    return w.Maxboard
}

// 获取一周的股票数据
func (w *OneWeek) GetArrDay() []OneDay {
    return w.ArrDay
}

// []排序
type ByDescending []int

func (a ByDescending) Len() int           { return len(a) }
func (a ByDescending) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDescending) Less(i, j int) bool { return a[i] > a[j] } // 注意这里的比较符号 降序排序

// 将excel的二维数组转为OneWeek结构体数据
func GetWriteSheet(writeSheet [][]string, sheetName string) {
    var weeker Weeker = firWeek
    weekNum := 0  //记录第几周
    maxBoard := 0 // 记录当前板最大板数

    changeRow := -1 // 切换到下一周时的行数，以防止反复切换下一周

    lastday := "" // 标记最后一天

    var curNum int64 = 0 // 记录当前是第几个

    for row, rows := range writeSheet {
        isNotStk := false // 不是股票信息的行
        var oneStk OneStk // 一条股票记录
        if row-1 >= 0 {
            if strings.TrimSpace(writeSheet[row-1][0]) == "板数" { // 板数下面一行也标为isNotStk
                isNotStk = true
            }
        }
        for col, cols := range rows {
            colStr := strings.TrimSpace(cols)
            if col == 0 && len(colStr) > 0 { // 第0列标记板数
                if colStr == "板数" {
                    if maxBoard > 0 {
                        weeker.SetParam(maxBoard)
                    }
                    isNotStk = true
                }
                curNum, _ = strconv.ParseInt(colStr, 10, 64)
                // 记录具体是第几板
                if curNum > 0 {
                    isNotStk = true
                    if maxBoard > 0 {
                        weeker.SetParam(maxBoard)
                    }
                }
                maxBoard = 0
            }
            // 记录日期
            if strings.HasPrefix(colStr, "202") && strings.Contains(colStr, "(周") {
                if changeRow != row {
                    // 切换到下一周
                    weekNum++
                    changeRow = row
                    switch weekNum {
                    case 1:
                        weeker = firWeek
                    case 2:
                        weeker = secWeek
                    case 3:
                        weeker = thridWeek
                    case 4:
                        weeker = fourWeek
                    case 5:
                        weeker = fifWeek
                    default:
                        break
                    }
                }
                strDate := colStr[:strings.Index(colStr, "(")]
                var oneDay OneDay
                oneDay.MapStk = make(map[int][]OneStk)
                oneDay.Date = strDate
                oneDay.WeekNum = time.Weekday(col/4 + 1)
                weeker.SetParam(oneDay)

                lastday = strDate
            }
            if col > 0 && !isNotStk {
                if col%4 == 1 {
                    strTmp := strings.TrimSpace(writeSheet[row-1][col]) // 上一行
                    if strings.HasPrefix(strTmp, "共") && strings.Contains(strTmp, "支") {
                        oneStk.PreRate = strTmp
                    }
                }
                switch col % 4 {
                case 0:
                    oneStk.Perform = colStr
                    // col最少从4开始进入这里
                    if len(oneStk.Name) > 0 {
                        weeker.PushStk(int(col/4)-1, int(curNum), oneStk)
                    }
                case 1:
                    oneStk.Name = colStr
                case 2:
                    oneStk.Money = convertChineseNumToDecimal(strings.TrimSpace(colStr))
                default:
                    oneStk.Reason = colStr
                }
            }
        }
        if curNum > 0 {
            maxBoard++
        }
    }
    if maxBoard > 0 {
        weeker.SetParam(maxBoard)
    }

    // 插入T+1日表现 -- begin
    tmpDate := weeker.GetArrDay()
    if len(tmpDate) > 0 {
        lastDate := tmpDate[len(tmpDate)-1]       // 取最后一天
        if preExcel.weekNum == lastDate.WeekNum { // 确保是同一天
            for k, v := range lastDate.MapStk { // 遍历第几板
                _, ok := preExcel.stkInfo[k]
                if ok {
                    for i, value := range v { // 遍历板下面的股票
                        for _, one := range preExcel.stkInfo[k] {
                            if one.Name == value.Name {
                                lastDate.MapStk[k][i].Code = one.Code
                                if one.changepercent+4 < 0.00000 {
                                    lastDate.MapStk[k][i].Perform = "大面"
                                } else if one.changepercent-0 < 0.00000 {
                                    lastDate.MapStk[k][i].Perform = "小面"
                                } else {
                                    tmp, ok := currExcel.stkInfo[k+1]
                                    if ok {
                                        for _, ttmp := range tmp {
                                            if ttmp.Name == one.Name {
                                                lastDate.MapStk[k][i].Perform = "晋级"
                                            }
                                        }
                                    }
                                    if lastDate.MapStk[k][i].Perform == "" {
                                        lastDate.MapStk[k][i].Perform = "红盘"
                                    }
                                }
                            }
                        }
                    }
                }

            }
        }
    }
    // 插入T+1日表现 -- end

    if IsMonFirstDay {
        WriteExcel(sheetName)
        firWeek.Init()
        secWeek.Init()
        thridWeek.Init()
        fourWeek.Init()
        fifWeek.Init()

        weeker = firWeek
        weekNum = 0
    }

    // 插入当日数据 -- begin
    var curDay OneDay // 当日数据

    // 1. 先把ExcelOrder 转换为 OneDay ////////////////////////////////////////////////////////////

    curDay.Date = fmt.Sprintf("%d.%02d.%02d", currExcel.year, currExcel.mon, currExcel.day)
    curDay.WeekNum = currExcel.weekNum
    curDay.MapStk = make(map[int][]OneStk)
    for k, v := range currExcel.stkInfo { // 遍历第几板
        for i, value := range v { // 遍历到单只股票
            var oneStk OneStk
            oneStk.Code = value.Code
            oneStk.Name = value.Name
            oneStk.Money = value.Money
            oneStk.Reason = value.Reason
            if i == 0 {
                if k == 1 {
                    oneStk.PreRate = fmt.Sprintf("共%d支", len(v))
                } else {
                    rate := float32(len(v)) / float32(len(preExcel.stkInfo[k-1])) * 100
                    oneStk.PreRate = fmt.Sprintf("共%d支 晋级率%.2f%%", len(v), rate)
                }
            }
            curDay.MapStk[k] = append(curDay.MapStk[k], oneStk)
        }
    }

    // ExcelOrder 转换为 OneDay -- end ////////////////////////////////////////////////////////////

    // 2. 将转换成功的OneDay数据插入到对应的周中，此处的难点在于要知道是第几周，而且还包括节假日、切换到下一周、
    //    一个月的第一天、当日连板数与这一周之前的天数不一致(少于或多于之前的连板数)....

    // 定义日期格式
    dateLayout := "2006.01.02"
    // 解析日期字符串
    lastdate, err := time.Parse(dateLayout, lastday)
    if err != nil {
        fmt.Println("Error parsing date:", lastday)
        return
    }
    curdate, err := time.Parse(dateLayout, curDay.Date)
    if err != nil {
        fmt.Println("Error parsing date:", curDay.Date)
        return
    }
    diff := curdate.Sub(lastdate)

    // 如果是放一天假不跨周末的那种呢 72小时 >= 两天，应该没有放两天假不跨周的吧
    if diff.Hours() >= 72 || IsMonFirstDay {
        //if curDay.WeekNum == time.Monday || IsMonFirstDay { // 周一的时候切换到下一周 想想节假日
        weekNum++
        switch weekNum {
        case 1:
            weeker = firWeek
        case 2:
            weeker = secWeek
        case 3:
            weeker = thridWeek
        case 4:
            weeker = fourWeek
        case 5:
            weeker = fifWeek
        default:
            break
        }
        keys := []int{}
        for k, _ := range currExcel.stkInfo {
            keys = append(keys, k)
        }
        sort.Sort(ByDescending(keys)) // 4 2 1 如果是不连续的怎么办
        // for i, v := range currExcel.stkInfo { // map的遍历是随机的
        if len(keys) > 0 {
            for k := keys[0]; k > 0; k-- { // [4 2 1]
                value, ok := currExcel.stkInfo[k] // 当日有这个板的数据
                if ok {
                    if len(value)+2 < 4 {
                        weeker.SetParam(4)
                        continue
                    }
                    weeker.SetParam(int(len(value) + 2))
                } else { // 当日没有这个板的数据，比如[4 2 1] 缺少3板的数据,但也要留出空行出来
                    weeker.SetParam(4)
                }
            }
            // 打印出当日的连板数组，方便确定数据有没有问题
            fmt.Println(weeker.GetArrDay())
        }
    } else { // 如果当天的数据很多，就可能需要改原来的maxBoard
        tmpSlice := make([]int, 0)
        arrBoard := weeker.GetMaxBoard() // [4 4 12 34]
        keys := []int{}
        for k, _ := range currExcel.stkInfo {
            keys = append(keys, k)
        }
        sort.Sort(ByDescending(keys)) // 4 2 1 如果是不连续的怎么办
        // for i, v := range currExcel.stkInfo { // map的遍历是随机的
        fmt.Println(keys)
        if len(keys) > 0 {
            max := 0
            if keys[0] > len(arrBoard) { // 两边的板数可能不一样，不一定谁多谁少
                max = keys[0]
            } else {
                max = len(arrBoard)
            }
            for k := max; k > 0; k-- { // [3 2 1] | [4 2 1]
                v, ok := currExcel.stkInfo[k] // 当日有这个板的数据
                if ok {
                    if len(arrBoard) < k { // 当日的板不在arrBoard里面 如 当日 [3 2 1] 之前的是 [2 1]
                        if len(v)+2 < 4 {
                            tmpSlice = append([]int{4}, tmpSlice...)
                            continue
                        }
                        // 从前面插入
                        tmpSlice = append([]int{len(v) + 2}, tmpSlice...)
                        continue
                    }
                    if arrBoard[len(arrBoard)-k] < len(v)+2 { // 当日的板数量大于之前的数量，按当日的数量来
                        tmpSlice = append(tmpSlice, len(v)+2)
                        continue
                    }
                    tmpSlice = append(tmpSlice, arrBoard[len(arrBoard)-k]) // 延续之前的数量
                } else { // 当日没有这个板的数据，比如[4 2 1] 缺少3板的数据,但也要留出空行出来
                    tmpSlice = append([]int{4}, tmpSlice...)
                }
            }
        }
        weeker.SetParam(tmpSlice)
        // 打印出当日的连板数组，方便确定数据有没有问题
        fmt.Println(tmpSlice)
    }
    weeker.SetParam(curDay)
    // 插入当日数据 -- end
}

// 将行列数转为excel里面特有的表格字符，如：第1行，第1列 ==> A1
// 特殊处理，按照星期几增加列数，这样就可以把每天的数据按照星期数固定在特定的列上
func GetExcelCol(row, col int, weekday time.Weekday) string {
    addcol := 0
    if weekday != time.Sunday && weekday != time.Saturday { // 周一到周五按照固定的列写入，防止节假日格式错乱的情况
        addcol = 4 * int(weekday-1)
    }
    str, _ := excelize.CoordinatesToCellName(col+addcol, row)
    // return fmt.Sprintf("%c%d", byte('A'+(col-1)%26), row)
    return str
}

// 按照firWeek,secWeek...写入excel表
func WriteExcel(sheetName string) bool {
    // 打开一个已存在的 Excel 文件
    f, err := excelize.OpenFile(excelPath)
    if err != nil {
        fmt.Printf("Error opening file: %v", err)
        return false
    }
    defer f.Close()

    // 获取前日数据
    readSheet := GetExcelInfo(excelPath, "涨跌停复盘")
    replayRow := len(readSheet)
    preDate := fmt.Sprintf("%d.%02d.%02d", preExcel.year, preExcel.mon, preExcel.day)
    curDate := fmt.Sprintf("%d.%02d.%02d", currExcel.year, currExcel.mon, currExcel.day)
    if !IsMonFirstDay {
        f.SetCellValue("涨跌停复盘", GetExcelCol(replayRow+1, 1, time.Sunday), curDate)
    }

    // 检查工作表是否存在，存在就删除
    i, err := f.GetSheetIndex(sheetName)
    if err == nil && i > 0 {
        f.DeleteSheet(sheetName)
        f.SaveAs(excelPath)
    }

    // 创建新Sheet
    f.NewSheet(sheetName)
    f.SetPanes(sheetName, &excelize.Panes{
        Freeze:      true,
        Split:       false,
        XSplit:      1,
        TopLeftCell: "A1",
        ActivePane:  "bottomLeft"}) // bottomLeft

    styleId, err := f.NewStyle(&excelize.Style{
        Alignment: &excelize.Alignment{
            Horizontal:      "center", // 水平对齐
            Indent:          0,        // 缩进
            JustifyLastLine: true,     // 两端对齐
            ReadingOrder:    0,        // 文字方向
            RelativeIndent:  0,        // 相对缩进
            ShrinkToFit:     false,    // 缩小字体
            TextRotation:    0,        // 文字旋转
            Vertical:        "center", // 垂直对齐
            WrapText:        false,    // 自动换行
        },
    })
    if err != nil {
        fmt.Println(err.Error())
        return false
    }

    // 左边对齐
    styleLeft, err := f.NewStyle(&excelize.Style{
        Alignment: &excelize.Alignment{
            Horizontal:      "left",   // 左边对齐
            Indent:          0,        // 缩进
            JustifyLastLine: true,     // 两端对齐
            ReadingOrder:    0,        // 文字方向
            RelativeIndent:  0,        // 相对缩进
            ShrinkToFit:     false,    // 缩小字体
            TextRotation:    0,        // 文字旋转
            Vertical:        "center", // 垂直对齐
            WrapText:        false,    // 自动换行
        },
    })
    if err != nil {
        fmt.Println(err.Error())
        return false
    }

    // 设置列宽
    // 参数分别为：工作表名，起始列，结束列（如果只设置一个列，则起始列和结束列相同），宽度值
    f.SetColWidth(sheetName, "D", "D", 40)
    f.SetColWidth(sheetName, "H", "H", 40)
    f.SetColWidth(sheetName, "L", "L", 40)
    f.SetColWidth(sheetName, "P", "P", 40)
    f.SetColWidth(sheetName, "T", "T", 40)

    f.SetColWidth(sheetName, "E", "E", 13)
    f.SetColWidth(sheetName, "I", "I", 13)
    f.SetColWidth(sheetName, "M", "M", 13)
    f.SetColWidth(sheetName, "Q", "Q", 13)
    f.SetColWidth(sheetName, "U", "U", 13)

    weekNum := 1 // 记录是第几周
    var weeker Weeker = firWeek

    row := 1 // excel的行数
MainLoop:
    for {
        // 取一周的数据
        arrDay := weeker.GetArrDay()
        boardSlice := weeker.GetMaxBoard()
        if len(boardSlice) == 0 { // 出口，maxBoard记录每个板的数量切片为0
            break
        }

        weekRow := row // 一周开始的行
        col := 1       // 一周开始的列

        rowTmp := weekRow
        // 写第一列
        f.SetCellValue(sheetName, GetExcelCol(rowTmp, col, time.Sunday), "板数")
        // 合并单元格
        f.MergeCell(sheetName, GetExcelCol(rowTmp, col, time.Sunday), GetExcelCol(rowTmp+1, col, time.Sunday))
        f.SetCellStyle(sheetName, GetExcelCol(rowTmp, col, time.Sunday), GetExcelCol(rowTmp+1, col, time.Sunday), styleId)

        rowTmp = rowTmp + 2
        for i, v := range boardSlice {
            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col, time.Sunday), fmt.Sprintf("%d", len(boardSlice)-i))
            f.MergeCell(sheetName, GetExcelCol(rowTmp, col, time.Sunday), GetExcelCol(rowTmp+v-1, col, time.Sunday))
            f.SetCellStyle(sheetName, GetExcelCol(rowTmp, col, time.Sunday), GetExcelCol(rowTmp+v-1, col, time.Sunday), styleId)
            rowTmp += v
        }

        col++
        // 多想一下，遇到节假日怎么办
        rowTmp = weekRow                // 调回到一周开始的地方
        for _, oneDay := range arrDay { // 按天写入
            // 2024.07.01(周一)
            strTmp := fmt.Sprintf("%s(周%d)", oneDay.Date, oneDay.WeekNum)
            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col, oneDay.WeekNum), strTmp)
            f.MergeCell(sheetName, GetExcelCol(rowTmp, col, oneDay.WeekNum), GetExcelCol(rowTmp, col+2, oneDay.WeekNum))

            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col+3, oneDay.WeekNum), "T+1日表现")
            f.SetCellStyle(sheetName, GetExcelCol(rowTmp, col, oneDay.WeekNum), GetExcelCol(rowTmp, col+3, oneDay.WeekNum), styleId)

            rowTmp++

            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col, oneDay.WeekNum), "名称")
            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col+1, oneDay.WeekNum), "封单额")
            f.SetCellValue(sheetName, GetExcelCol(rowTmp, col+2, oneDay.WeekNum), "涨停原因")
            f.SetCellStyle(sheetName, GetExcelCol(rowTmp, col, oneDay.WeekNum), GetExcelCol(rowTmp, col+2, oneDay.WeekNum), styleId)

            // 标记特殊的行，之后写入大面概率用的
            specialRow := rowTmp
            tmpNum := 0   // 记录大面的股票数量
            totalNum := 0 // 记录二板及二板以上的股票总数

            rowTmp++

            // 有点复杂啊
            for pos, v := range boardSlice { // 按板数遍历
                arrStk, ok := oneDay.MapStk[len(boardSlice)-pos]
                index := rowTmp
                if ok {
                    if curDate == oneDay.Date && !IsMonFirstDay {
                        f.SetCellValue("涨跌停复盘", GetExcelCol(replayRow+1, 12+len(boardSlice)-pos, time.Sunday), fmt.Sprintf("%d", len(arrStk)))
                    }
                    // 遍历股票
                    for i, v := range arrStk {
                        if len(boardSlice)-pos > 1 {
                            if v.Perform == "大面" {
                                tmpNum++
                            }
                            totalNum++
                        }
                        if i == 0 {
                            // 共4支 晋级率16.00%
                            f.SetCellValue(sheetName, GetExcelCol(index, col, oneDay.WeekNum), v.PreRate)
                            f.MergeCell(sheetName, GetExcelCol(index, col, oneDay.WeekNum), GetExcelCol(index, col+2, oneDay.WeekNum))
                            f.SetCellStyle(sheetName, GetExcelCol(index, col, oneDay.WeekNum), GetExcelCol(index, col+2, oneDay.WeekNum), styleId)
                        }
                        f.SetCellValue(sheetName, GetExcelCol(index+1, col, oneDay.WeekNum), v.Name)
                        f.SetCellValue(sheetName, GetExcelCol(index+1, col+1, oneDay.WeekNum), v.Money)
                        f.SetCellValue(sheetName, GetExcelCol(index+1, col+2, oneDay.WeekNum), v.Reason)
                        if len(v.Perform) > 0 {
                            f.SetCellValue(sheetName, GetExcelCol(index+1, col+3, oneDay.WeekNum), v.Perform)
                        } else {
                            if strings.HasPrefix(v.Code, "3") {
                                f.SetCellValue(sheetName, GetExcelCol(index+1, col+3, oneDay.WeekNum), "创")
                            } else if strings.HasPrefix(v.Code, "688") {
                                f.SetCellValue(sheetName, GetExcelCol(index+1, col+3, oneDay.WeekNum), "科")
                            }
                        }

                        f.SetCellStyle(sheetName, GetExcelCol(index+1, col, oneDay.WeekNum), GetExcelCol(index+1, col+3, oneDay.WeekNum), styleLeft)
                        index++
                    }
                }
                rowTmp += v
            }

            f.SetCellValue(sheetName, GetExcelCol(specialRow, col+3, oneDay.WeekNum), fmt.Sprintf("大面概率:%.f%%", float32(tmpNum)/float32(totalNum)*100))
            f.SetCellStyle(sheetName, GetExcelCol(specialRow, col+3, oneDay.WeekNum), GetExcelCol(specialRow, col+3, oneDay.WeekNum), styleLeft)

            if preDate == oneDay.Date && !IsMonFirstDay { // 写前一日的大面概率
                str := fmt.Sprintf("%.2f%%", float32(tmpNum)/float32(totalNum)*100)
                f.SetCellValue("涨跌停复盘", GetExcelCol(replayRow, 12, time.Sunday), str)
            }
            if curDate == oneDay.Date && !IsMonFirstDay { // 写当日的连板数量
                f.SetCellValue("涨跌停复盘", GetExcelCol(replayRow+1, 8, time.Sunday), totalNum)
            }

            rowTmp = weekRow // 调回一周开始的位置
        }

        // 调到下一周的行
        // [4 4 4 4 4 4 4 5 7 14 45]
        for _, v := range boardSlice {
            row += v
        }
        row += 3
        // 切换到下一周
        weekNum++
        switch weekNum {
        case 1:
            weeker = firWeek
        case 2:
            weeker = secWeek
        case 3:
            weeker = thridWeek
        case 4:
            weeker = fourWeek
        case 5:
            weeker = fifWeek
        default:
            break MainLoop
        }
    }
    if !IsMonFirstDay {
        f.DeleteSheet("Sheet1")
        f.DeleteSheet("Sheet2")
    }
    f.SaveAs(excelPath)
    return true
}
