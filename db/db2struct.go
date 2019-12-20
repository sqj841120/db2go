package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

//map for converting mysql type to golang types
var typeForMysqlToGo = map[string]string{
	"int":                "util.Long",
	"integer":            "util.Long",
	"tinyint":            "util.Long",
	"smallint":           "util.Long",
	"mediumint":          "util.Long",
	"bigint":             "util.Long",
	"int unsigned":       "util.Long",
	"integer unsigned":   "util.Long",
	"tinyint unsigned":   "util.Long",
	"smallint unsigned":  "util.Long",
	"mediumint unsigned": "util.Long",
	"bigint unsigned":    "util.Long",
	"bit":                "util.Long",
	"bool":               "bool",
	"enum":               "string",
	"set":                "string",
	"varchar":            "string",
	"char":               "string",
	"tinytext":           "string",
	"mediumtext":         "string",
	"text":               "string",
	"longtext":           "string",
	"blob":               "string",
	"tinyblob":           "string",
	"mediumblob":         "string",
	"longblob":           "string",
	"date":               "date.Datetime", // date.Datetime or string
	"datetime":           "date.Datetime", // date.Datetime or string
	"timestamp":          "date.Datetime", // date.Datetime or string
	"time":               "date.Datetime", // date.Datetime or string
	"float":              "float64",
	"double":             "float64",
	"decimal":            "float64",
	"binary":             "string",
	"varbinary":          "string",
}

type Db2Struct struct {
	dsn            string
	savePath       string
	db             *sql.DB
	table          string
	prefix         string
	config         *T2tConfig
	err            error
	realNameMethod string
	enableJsonTag  bool   // 是否添加json的tag, 默认不添加
	packageName    string // 生成struct的包名(默认为空的话, 则取名为: package model)
	tagKey         string // tag字段的key值,默认是orm
	dateToTime     bool   // 是否将 date相关字段转换为 date.Datetime,默认否
}

type T2tConfig struct {
	StructNameToHump bool // 结构体名称是否转为驼峰式，默认为false
	RmTagIfUcFirsted bool // 如果字段首字母本来就是大写, 就不添加tag, 默认false添加, true不添加
	TagToLower       bool // tag的字段名字是否转换为小写, 如果本身有大写字母的话, 默认false不转
	//JsonTagToHump    bool // json tag是否转为驼峰，默认为false，不转换
	UcFirstOnly bool // 字段首字母大写的同时, 是否要把其他字母转换为小写,默认false不转换
	SeperatFile bool // 每个struct放入单独的文件,默认false,放入同一个文件
}

func NewDb2Struct() *Db2Struct {
	return &Db2Struct{}
}

func (t *Db2Struct) Dsn(d string) *Db2Struct {
	t.dsn = d
	return t
}

func (t *Db2Struct) TagKey(r string) *Db2Struct {
	t.tagKey = r
	return t
}

func (t *Db2Struct) PackageName(r string) *Db2Struct {
	t.packageName = r
	return t
}

func (t *Db2Struct) RealNameMethod(r string) *Db2Struct {
	t.realNameMethod = r
	return t
}

func (t *Db2Struct) SavePath(p string) *Db2Struct {
	t.savePath = p
	return t
}

func (t *Db2Struct) DB(d *sql.DB) *Db2Struct {
	t.db = d
	return t
}

func (t *Db2Struct) Table(tab string) *Db2Struct {
	t.table = tab
	return t
}

func (t *Db2Struct) Prefix(p string) *Db2Struct {
	t.prefix = p
	return t
}

func (t *Db2Struct) EnableJsonTag(p bool) *Db2Struct {
	t.enableJsonTag = p
	return t
}

func (t *Db2Struct) DateToTime(d bool) *Db2Struct {
	t.dateToTime = d
	return t
}

func (t *Db2Struct) Config(c *T2tConfig) *Db2Struct {
	t.config = c
	return t
}

func (t *Db2Struct) Run() error {
	if t.config == nil {
		t.config = new(T2tConfig)
	}
	// 链接mysql, 获取db对象
	t.dialMysql()
	if t.err != nil {
		return t.err
	}

	// 获取表和字段的shcema
	tableColumns, err := t.getColumns()
	if err != nil {
		return err
	}

	// 包名
	var packageName string
	if t.packageName == "" {
		packageName = "package model\n\n"
	} else {
		packageName = fmt.Sprintf("package %s\n\n", t.packageName)
	}

	for tableRealName, item := range tableColumns {
		// 组装struct
		var structContent string
		// 去除前缀
		if t.prefix != "" {
			tableRealName = tableRealName[len(t.prefix):]
		}
		tableName := t.camelCase(tableRealName)

		depth := 1
		structContent += "type " + tableName + " struct {\n"
		for _, v := range item {
			// 字段注释
			var clumnComment string
			if v.ColumnComment != "" {
				clumnComment = fmt.Sprintf(" // %s", v.ColumnComment)
			}
			structContent += fmt.Sprintf("%s%s %s %s%s\n", tab(depth), v.ColumnName, v.Type, v.Tag, clumnComment)
		}
		structContent += tab(depth-1) + "}\n\n"

		// 添加 method 获取真实表名
		if t.realNameMethod != "" {
			structContent += fmt.Sprintf("func (%s) %s() string {\n", tableName, t.realNameMethod)
			structContent += fmt.Sprintf("%sreturn \"%s\"\n", tab(depth), tableRealName)
			structContent += "}\n\n"
		}

		//newStruct
		structContent += fmt.Sprintf("func New%s() %s {\n", tableName, tableName)
		structContent += fmt.Sprintf("	that := %s{}\n", tableName)
		structContent += fmt.Sprintf("	that.GenId()\n")
		structContent += fmt.Sprintf("	return that\n")
		structContent += "}\n\n"

		//GenId
		structContent += fmt.Sprintf("func (that *%s) GenId() {\n", tableName)
		structContent += fmt.Sprintf("	that.Id = util.NextId()\n")
		structContent += "}\n\n"

		//UnmarshalJSON 反序列化
		structContent += fmt.Sprintf("	// 反序列化Json的时候，如果没有ID，自动给一个\n")
		structContent += fmt.Sprintf("func (that *%s) UnmarshalJSON(value []byte) error {\n", tableName)
		structContent += fmt.Sprintf("	type Alias %s\n", tableName)
		structContent += fmt.Sprintf("	alias := &struct {\n")
		structContent += fmt.Sprintf("		*Alias\n")
		structContent += fmt.Sprintf("	}{Alias: (*Alias)(that)}\n")
		structContent += fmt.Sprintf("	if err := json.Unmarshal(value, &alias); err != nil {\n")
		structContent += fmt.Sprintf("		return err\n")
		structContent += fmt.Sprintf("	}\n")
		structContent += fmt.Sprintf("	if nil != alias && alias.Id == 0 {\n")
		structContent += fmt.Sprintf("		alias.Id = util.NextId()\n")
		structContent += fmt.Sprintf("	}\n")
		structContent += fmt.Sprintf("	return nil\n")
		structContent += "}\n\n"

		//UnmarshalXML 反序列化XML
		structContent += fmt.Sprintf("	// 反序列化XML的时候，如果没有ID，自动给一个\n")
		structContent += fmt.Sprintf("func (that *%s) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {\n", tableName)
		structContent += fmt.Sprintf("	type Alias %s\n", tableName)
		structContent += fmt.Sprintf("	alias := &struct {\n")
		structContent += fmt.Sprintf("		*Alias\n")
		structContent += fmt.Sprintf("	}{Alias: (*Alias)(that)}\n")
		structContent += fmt.Sprintf("	if err := d.DecodeElement(&alias, &start); err != nil {\n")
		structContent += fmt.Sprintf("		return err\n")
		structContent += fmt.Sprintf("	}\n")
		structContent += fmt.Sprintf("	if nil != alias && alias.Id == 0 {\n")
		structContent += fmt.Sprintf("		alias.Id = util.NextId()\n")
		structContent += fmt.Sprintf("	}\n")
		structContent += fmt.Sprintf("	return nil\n")
		structContent += "}\n\n"
		//fmt.Println(structContent)

		var importContent string
		importContent += fmt.Sprintf("import (\n")
		importContent += fmt.Sprintf("	\"encoding/json\"\n")
		importContent += fmt.Sprintf("	\"encoding/xml\"\n")
		// 如果有引入 util.Long, 则需要引入 clc.com/go_bi/util 包
		if strings.Contains(structContent, "util.Long") {
			importContent += fmt.Sprintf("	\"clc.com/go_bi/util\"\n")
		}
		// 如果有引入 date.Datetime, 则需要引入 clc.com/go_bi/util/date 包
		if strings.Contains(structContent, "date.Datetime") {
			importContent += " 	\"clc.com/go_bi/util/date\"\n"
		}
		importContent += fmt.Sprintf(")\n")

		// 写入文件struct
		var savePath string
		// 是否指定保存路径
		if t.savePath == "" {
			savePath = fmt.Sprintf("./model/%s.go", tableName)
		} else {
			savePath = fmt.Sprintf(t.savePath+"/%s.go", tableName)
		}
		f, err := os.Create(savePath)
		if err != nil {
			fmt.Println("Can not write file")
			return err
		}
		defer f.Close()

		f.WriteString(packageName + importContent + structContent)

		cmd := exec.Command("gofmt", "-w", savePath)
		cmd.Run()
	}
	return nil
}

func (t *Db2Struct) dialMysql() {
	if t.db == nil {
		if t.dsn == "" {
			t.err = errors.New("dsn数据库配置缺失")
			return
		}
		t.db, t.err = sql.Open("mysql", t.dsn)
	}
}

type column struct {
	ColumnName    string
	Type          string
	Nullable      string
	TableName     string
	ColumnComment string
	Tag           string
	//Json          string
}

// Function for fetching schema definition of passed table
func (t *Db2Struct) getColumns(table ...string) (tableColumns map[string][]column, err error) {
	// 根据设置,判断是否要把 date 相关字段替换为 date.Datetime
	if !t.dateToTime {
		typeForMysqlToGo["date"] = "date.Datetime"
		typeForMysqlToGo["datetime"] = "date.Datetime"
		typeForMysqlToGo["timestamp"] = "date.Datetime"
		typeForMysqlToGo["time"] = "date.Datetime"
	}
	tableColumns = make(map[string][]column)
	// sql
	var sqlStr = `SELECT COLUMN_NAME,DATA_TYPE,IS_NULLABLE,TABLE_NAME,COLUMN_COMMENT
		FROM information_schema.COLUMNS 
		WHERE table_schema = DATABASE()`
	// 是否指定了具体的table
	if t.table != "" {
		sqlStr += fmt.Sprintf(" AND TABLE_NAME = '%s'", t.prefix+t.table)
	}
	// sql排序
	sqlStr += " order by TABLE_NAME asc, ORDINAL_POSITION asc"

	rows, err := t.db.Query(sqlStr)
	if err != nil {
		fmt.Println("Error reading table information: ", err.Error())
		return
	}

	defer rows.Close()

	for rows.Next() {
		col := column{}
		err = rows.Scan(&col.ColumnName, &col.Type, &col.Nullable, &col.TableName, &col.ColumnComment)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		//col.Json = strings.ToLower(col.ColumnName)
		col.Tag = col.ColumnName
		//col.ColumnComment = col.ColumnComment
		col.ColumnName = t.camelCase(col.ColumnName)
		col.Type = typeForMysqlToGo[col.Type]
		//jsonTag := col.Tag
		jsonTag := strings.ToLower(col.ColumnName[0:1]) + col.ColumnName[1:]
		// 字段首字母本身大写, 是否需要删除tag
		if t.config.RmTagIfUcFirsted &&
			col.ColumnName[0:1] == strings.ToUpper(col.ColumnName[0:1]) {
			col.Tag = "-"
		} else {
			// 是否需要将tag转换成小写
			if t.config.TagToLower {
				col.Tag = strings.ToLower(col.Tag)
			}
		}
		if t.tagKey == "" {
			t.tagKey = "xorm"
		}
		if t.enableJsonTag {
			col.Tag = fmt.Sprintf("`%s:\"%s\" json:\"%s\"`", t.tagKey, col.Tag, jsonTag)
		} else {
			col.Tag = fmt.Sprintf("`%s:\"%s\"`", t.tagKey, col.Tag)
		}
		if _, ok := tableColumns[col.TableName]; !ok {
			tableColumns[col.TableName] = []column{}
		}
		tableColumns[col.TableName] = append(tableColumns[col.TableName], col)
	}
	return
}

func (t *Db2Struct) camelCase(str string) string {
	// 是否有表前缀, 设置了就先去除表前缀
	if t.prefix != "" {
		str = strings.Replace(str, t.prefix, "", 1)
	}
	var text string
	//for _, p := range strings.Split(name, "_") {
	for _, p := range strings.Split(str, "_") {
		// 字段首字母大写的同时, 是否要把其他字母转换为小写
		switch len(p) {
		case 0:
		case 1:
			text += strings.ToUpper(p[0:1])
		default:
			// 字符长度大于1时
			if t.config.UcFirstOnly {
				text += strings.ToUpper(p[0:1]) + strings.ToLower(p[1:])
			} else {
				text += strings.ToUpper(p[0:1]) + p[1:]
			}
		}
	}
	return text
}
func tab(depth int) string {
	return strings.Repeat("\t", depth)
}
