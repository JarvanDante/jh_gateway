package registry

import (
	"context"
	"fmt"
	"log"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/hashicorp/consul/api"
)

var client *api.Client

func InitConsul() {
	cfg := api.DefaultConfig()
	addr := g.Cfg().MustGet(context.Background(), "consul.addr").String()
	cfg.Address = addr

	var err error
	client, err = api.NewClient(cfg)
	if err != nil {
		log.Fatalf("init consul failed: %v", err)
	}
}

func GetServiceAddr(serviceName string) (string, error) {
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil || len(services) == 0 {
		return "", fmt.Errorf("no healthy instance for service: %s", serviceName)
	}
	// 简单取第一个，可以改成随机 / 轮询
	svc := services[0].Service
	return fmt.Sprintf("%s:%d", svc.Address, svc.Port), nil
}
