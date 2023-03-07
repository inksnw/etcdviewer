package main

import (
	"etcdkeeper/lib"
	"github.com/gin-gonic/gin"
)

func main() {
	lib.InitConfig()
	lib.InitClient()
	lib.InitSch()
	r := gin.Default()
	r.Static("/ui", "./assets/")
	r.GET("/v3/connect", lib.Connect)
	r.GET("/v3/get", lib.Get)
	r.GET("/v3/getpath", lib.GetPath)
	r.Run()
}
