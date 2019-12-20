package main

import (
	"fmt"
	"sqj/db2go/tool"
)

func main() {
	runDb2Struct()
	Run2Res()

}
func Run2Res() {
	obj := tool.NewModel2Res()
	obj.Run()
}
func Run2Rq() {
	obj := tool.NewModel2Rq()
	obj.Run()
}

func runDb2Struct() {
	obj := tool.NewDb2Struct()

	//err := obj.
	//	SavePath("/home/go/project/model/model.go").
	//	Dsn("root:root@tcp(localhost:3306)/test?charset=utf8").
	//	Run()
	// 个性化配置

	err := obj.
		Config(&tool.T2tConfig{
			TagToLower: false,
			//StructNameToHump bool // 结构体名称是否转为驼峰式，默认为false
			//RmTagIfUcFirsted bool // 如果字段首字母本来就是大写, 就不添加tag, 默认false添加, true不添加
			//TagToLower       bool // tag的字段名字是否转换为小写, 如果本身有大写字母的话, 默认false不转
			//JsonTagToHump    bool // json tag是否转为驼峰，默认为false，不转换
			//UcFirstOnly      bool // 字段首字母大写的同时, 是否要把其他字母转换为小写,默认false不转换
			//SeperatFile      bool // 每个struct放入单独的文件,默认false,放入同一个文件
		}).
		// 指定某个表,如果不指定,则默认全部表都迁移
		//Table("user").
		// 表前缀
		//Prefix("prefix_").
		// 是否添加json tag
		//EnableJsonTag(true).
		// 生成struct的包名(默认为空的话, 则取名为: package model)
		PackageName("model").
		// tag字段的key值,默认是xorm
		//TagKey("xorm").
		// 是否添加结构体方法获取表名
		RealNameMethod("TableName").
		EnableJsonTag(true).
		// 生成的结构体保存路径
		//SavePath("./model").
		// 数据库dsn,这里可以使用 t2t.DB() 代替,参数为 *sql.DB 对象
		Dsn("root:root@tcp(192.168.1.192:3306)/bi?charset=utf8").
		// 执行
		Run()
	fmt.Println(err)
}
