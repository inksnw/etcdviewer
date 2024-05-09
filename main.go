package main

import (
	"etcdkeeper/lib"
	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
	"os"
)

func main() {
	initLog()
	lib.InitConfig()
	lib.InitClient()
	lib.InitSch()
	r := gin.Default()
	r.Static("/ui", "./assets/")
	r.GET("/v3/get", lib.Get)
	r.GET("/v3/getpath", lib.GetPath)
	addr := "0.0.0.0:8000"
	log.Info().Msgf("listen on http://%s/ui", addr)
	r.Run(addr)
}

func initLog() {
	gin.SetMode(gin.ReleaseMode)
	if !log.IsTerminal(os.Stderr.Fd()) {
		return
	}
	log.DefaultLogger = log.Logger{
		TimeFormat: "15:04:05",
		Caller:     1,
		Writer: &log.ConsoleWriter{
			ColorOutput:    true,
			QuoteString:    true,
			EndWithMessage: true,
		},
	}

}
