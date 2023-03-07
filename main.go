package main

import (
	"etcdkeeper/lib"
	"github.com/gin-gonic/gin"
)

func main() {
	lib.InitClient()
	r := gin.Default()
	r.Static("/etcdkeeper", "./assets/etcdkeeper")
	r.Static("/framework", "./assets/framework")
	r.POST("/v3/connect", lib.Connect)
	r.GET("/v3/get", lib.Get)
	r.GET("/v3/getpath", lib.GetPath)
	r.Run()
}
