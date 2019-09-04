package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"strconv"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/logger"
	"github.com/kataras/iris/middleware/recover"
	"github.com/iris-contrib/middleware/cors"
	//"github.com/kataras/iris/mvc"
	//"gopkg.in/go-playground/validator.v9"
	"github.com/kataras/iris/sessions"

	"gosync/library"
)

var sessionManager *sessions.Sessions

func main() {
	var cookieNameForSessionID = "gosync_sessionid"
	sessionManager = sessions.New(sessions.Config{Cookie: cookieNameForSessionID,Expires:30*time.Minute})
	//current_path, _ := filepath.Abs(`.`)
	var GOPATH = os.Getenv("GOPATH")
	arg_list := os.Args
	var config_file string
	var PROJECT_DIR = GOPATH + "/src/gosync/"
	if len(arg_list) >1 {
		config_file = arg_list[1]
	}else {
		config_file = PROJECT_DIR + "config/main.tml"
	}
	if !library.IsFile(config_file) {
		fmt.Println("config file: "+config_file+" not exists")
		os.Exit(1)
	}
	var(
		config *library.TomlConfig
		err error
	)
	if config, err = library.ReadConf(config_file); err != nil {
		checkErr(err)
	}

	var FAB_FILE = config.Fabric.FabFile
	log_path := config.Logs.LogPath
	static_path := config.Static.StaticPath
	view_path := config.Views.ViewPath
	server_host := config.Server.Host
	server_port := config.Server.Port
	server_port_str := strconv.FormatInt(int64(server_port),10)
	host_url := "http://"+server_host+":"+server_port_str
	if !library.IsDir(static_path) {
		fmt.Println("static path: "+static_path+" not exists")
		os.Exit(1)
	}
	if !library.IsDir(view_path) {
		fmt.Println("views path: "+view_path+" not exists")
		os.Exit(1)
	}
	if library.Substr(log_path,-1,1) != "/" {
		log_path = log_path + "/"
	}
	if library.Substr(view_path,-1,1) != "/" {
		view_path = view_path + "/"
	}

	app := iris.New()
	// Recover middleware recovers from any panics and writes a 500 if there was one.
	app.Use(recover.New())

	requestLogger := logger.New(logger.Config{
		Status: true,
		IP: true,
		Method: true,
		Path: true,
		Query: true,
		// if !empty then its contents derives from `ctx.Values().Get("logger_message")
		// will be added to the logs.
		MessageContextKeys: []string{"logger_message"},
		// if !empty then its contents derives from `ctx.GetHeader("User-Agent")
		MessageHeaderKeys: []string{"User-Agent"},
	})
	app.Use(requestLogger)

	f := newLogFile(log_path)
	defer f.Close()

	app.Logger().SetOutput(f)

	//错误处理handle 404 500
	app.OnErrorCode(iris.StatusNotFound, notFound)
	app.OnErrorCode(iris.StatusInternalServerError, internalServerError)

	app.Use(before)
	// 注册  "after" ，在所有路由的处理程序之后调用
	app.Done(after)

	crs := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081"},   //允许通过的主机名称
		AllowCredentials: true,
	})

	app.StaticWeb("/static", static_path)

	app.RegisterView(iris.HTML(view_path, ".html"))
	app.Get("/", func(ctx iris.Context) {
		session := sessionManager.Start(ctx)
		username := session.GetString("username")
		if username=="" {
			ctx.Redirect("/login",http.StatusFound)
		}


		ctx.ViewData("username", username)
		ctx.ViewData("host_url", host_url)
		ctx.ViewData("title", "gosync上线系统")
		ctx.View("index.html")
	},crs)
	app.Get("/login", func(ctx iris.Context) {
		session := sessionManager.Start(ctx)
		username := session.GetString("username")
		if username!="" {
			ctx.Redirect("/",http.StatusFound)
		}
		ctx.ViewData("host_url", host_url)
		ctx.View("login.html")
	})
	app.Get("/logout",func(ctx iris.Context){
		session := sessionManager.Start(ctx)
		session.Delete("username")
		time.Sleep(time.Duration(500) * time.Millisecond)
		ctx.Redirect("/login",http.StatusFound)
	})

	app.Post("/api/login", func(ctx iris.Context) {
		username := ctx.FormValue("username")
		password := ctx.FormValue("password")

		real_user := "gosync"
		real_pwd := "123456"

		customer_user := "customer"
		customer_pwd := "wangtuo888"

		if ((username!= real_user || password!= real_pwd) && (username!= customer_user || password!= customer_pwd)) {
			ctx.WriteString("Error: 用户密码错误 !!!")
			return
		}
		session := sessionManager.Start(ctx)
		session.Set("username", username)
		ctx.WriteString("登录成功")
	})

	api := app.Party("/api",corsAll)
	{
		api.Get("/tag_list/{project:string}",func(ctx iris.Context){
			project := ctx.Params().Get("project")
			rs,err := exec_shell("fab -r "+FAB_FILE+" go-tag-list "+project)
			if err !=nil {
				ctx.WriteString(err.Error())
			}else {
				sr := strings.Trim(rs,"\n")
				//创建切片
				tag_list := make([]string,0,1)
				tag_list = strings.Split(sr,"\n")
				library.ReverseArray(tag_list)
				rs = strings.Join(tag_list,"\n")
				ctx.WriteString(rs)
			}
		})
		api.Get("/create_tag/{project:string}/{tag:string}",func(ctx iris.Context){
			project := ctx.Params().Get("project")
			tag := ctx.Params().Get("tag")
			tag = strings.Trim(tag," ")
			rs,err := exec_shell("fab -r "+FAB_FILE+" create-tag "+project+" "+tag)
			if err !=nil {
				ctx.WriteString(err.Error())
			}else {
				ctx.WriteString(rs)
			}
		})
		api.Get("/testing/{project:string}",func(ctx iris.Context){
			project := ctx.Params().Get("project")
			rs,err := exec_shell("fab -r "+FAB_FILE+" testing "+project)
			if err !=nil {
				ctx.WriteString(err.Error())
			}else {
				ctx.WriteString(rs)
			}
		})
		api.Get("/deploy/{project:string}",func(ctx iris.Context){
			project := ctx.Params().Get("project")
			rs,err := exec_shell("fab -r "+FAB_FILE+" deploy "+project)
			if err !=nil {
				ctx.WriteString(err.Error())
			}else {
				ctx.WriteString(rs)
			}
		})
		api.Get("/rollback/{project:string}",func(ctx iris.Context){
			project := ctx.Params().Get("project")
			rs,err := exec_shell("fab -r "+FAB_FILE+" rollback "+project)
			if err !=nil {
				ctx.WriteString(err.Error())
			}else {
				ctx.WriteString(rs)
			}
		})
	}

	_ = app.Run(iris.Addr(":"+server_port_str),iris.WithConfiguration(iris.TOML(config_file)))
}

func newLogFile(log_path string) *os.File {
	filename := log_path +strconv.Itoa(time.Now().Year()) + time.Now().Month().String() + strconv.Itoa(time.Now().Day()) +".log"
	// Open the file, this will append to the today's file if server restarted.
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return f
}

func before(ctx iris.Context) {
	//
	//fmt.Println("before request ")
	ctx.Next()
}
func after(ctx iris.Context) {
	//fmt.Println("after request ")
	ctx.Next()
}

func notFound(ctx iris.Context) {
	// 出现 404 的时候，就跳转到 $views_dir/errors/404.html 模板
	//ctx.View("errors/404.html")
	ctx.WriteString("The page not found !!!")
}

func internalServerError(ctx iris.Context) {
	ctx.WriteString("Oups something went wrong, try again")
}

//阻塞式的执行外部shell命令的函数,等待执行完毕并返回标准输出
func exec_shell(s string) (string, error){
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	cmd := exec.Command("/bin/bash", "-c", s)

	//读取io.Writer类型的cmd.Stdout，再通过bytes.Buffer(缓冲byte类型的缓冲器)将byte类型转化为string类型(out.String():这是bytes类型提供的接口)
	var out bytes.Buffer
	cmd.Stdout = &out

	//Run执行c包含的命令，并阻塞直到完成。  这里stdout被取出，cmd.Wait()无法正确获取stdin,stdout,stderr，则阻塞在那了
	err := cmd.Run()
	checkErr(err)
	return out.String(), err
}
//错误处理函数
func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}

func corsAll(ctx iris.Context) {
	//cors.AllowAll()
	session := sessionManager.Start(ctx)
	username := session.GetString("username")
	if username=="" {
		ctx.WriteString("Error: 请先登录 !!!")
		return
	}
	ctx.Next()
}