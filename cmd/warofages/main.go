package main

import (
	"warofages/internal/server"
	"warofages/internal/util"
)

func main() {
	conf := util.LoadConfig()
	server.StartServer(conf)
}
