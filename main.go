package main

import (
	"jh_gateway/internal/registry"
	"jh_gateway/internal/router"

	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	// 初始化 Consul 客户端（可选，不用 Consul 的话改成固定地址）
	registry.InitConsul()

	s := g.Server()
	router.Register(s)

	s.Run()
}
