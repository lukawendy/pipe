// Pipe - A small and beautiful blogging platform written in golang.
// Copyright (C) 2017-2019, b3log.org & hacpai.com
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/b3log/pipe/controller"
	"github.com/b3log/pipe/cron"
	"github.com/b3log/pipe/i18n"
	"github.com/b3log/pipe/log"
	"github.com/b3log/pipe/model"
	"github.com/b3log/pipe/service"
	"github.com/b3log/pipe/theme"
	"github.com/b3log/pipe/util"
	"github.com/gin-gonic/gin"
)

// Logger
var logger *log.Logger

// The only one init function in pipe.
func init() {
	// 随机数种子
	rand.Seed(time.Now().UTC().UnixNano())
	// 设置日志级别
	log.SetLevel("warn")
	// 设置日志输出
	logger = log.NewLogger(os.Stdout)
	// 加载配置
	model.LoadConf()
	util.LoadMarkdown()
	i18n.Load()
	theme.Load()
	// 替换服务地址配置
	replaceServerConf()

	if "dev" == model.Conf.RuntimeMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	gin.DefaultWriter = io.MultiWriter(os.Stdout)
}

// Entry point.
func main() {
	service.ConnectDB()
	service.Upgrade.Perform()
	// 启动定时任务
	cron.Start()

	router := controller.MapRoutes()
	server := &http.Server{
		Addr:    "0.0.0.0:" + model.Conf.Port,
		Handler: router,
	}

	handleSignal(server)

	logger.Infof("Pipe (v%s) is running [%s]", model.Version, model.Conf.Server)
	if err := server.ListenAndServe(); nil != err {
		logger.Fatalf("listen and serve failed: " + err.Error())
	}
}

// handleSignal handles system signal for graceful shutdown.
func handleSignal(server *http.Server) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	go func() {
		s := <-c
		logger.Infof("got signal [%s], exiting pipe now", s)
		if err := server.Close(); nil != err {
			logger.Errorf("server close failed: " + err.Error())
		}

		service.DisconnectDB()

		logger.Infof("Pipe exited")
		os.Exit(0)
	}()
}

func replaceServerConf() {
	path := "theme/sw.min.js.tpl"
	if util.File.IsExist(path) {
		data, err := ioutil.ReadFile(path)
		if nil != err {
			logger.Fatal("read file [" + path + "] failed: " + err.Error())
		}
		content := string(data)
		content = strings.Replace(content, "http://server.tpl.json", model.Conf.Server, -1)
		content = strings.Replace(content, "http://staticserver.tpl.json", model.Conf.StaticServer, -1)
		content = strings.Replace(content, "${StaticResourceVersion}", model.Conf.StaticResourceVersion, -1)
		writePath := strings.TrimSuffix(path, ".tpl")
		if err = ioutil.WriteFile(writePath, []byte(content), 0644); nil != err {
			logger.Fatal("replace sw.min.js in [" + path + "] failed: " + err.Error())
		}
	}

	if util.File.IsExist("console/dist/") {
		err := filepath.Walk("console/dist/", func(path string, f os.FileInfo, err error) error {
			if strings.HasSuffix(path, ".tpl") {
				data, err := ioutil.ReadFile(path)
				if nil != err {
					logger.Fatal("read file [" + path + "] failed: " + err.Error())
				}
				content := string(data)
				content = strings.Replace(content, "http://server.tpl.json", model.Conf.Server, -1)
				content = strings.Replace(content, "http://staticserver.tpl.json", model.Conf.StaticServer, -1)
				writePath := strings.TrimSuffix(path, ".tpl")
				if err = ioutil.WriteFile(writePath, []byte(content), 0644); nil != err {
					logger.Fatal("replace server conf in [" + writePath + "] failed: " + err.Error())
				}
			}

			return err
		})
		if nil != err {
			logger.Fatal("replace server conf in [theme] failed: " + err.Error())
		}
	}
}
