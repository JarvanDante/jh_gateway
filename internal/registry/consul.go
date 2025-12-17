package registry

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/hashicorp/consul/api"
)

var (
	client *api.Client
	// 轮询计数器，用于轮询负载均衡
	roundRobinCounters = make(map[string]*int64)
	counterMutex       = sync.RWMutex{}
)

func InitConsul() {
	cfg := api.DefaultConfig()
	addr := g.Cfg().MustGet(context.Background(), "consul.addr").String()
	cfg.Address = addr

	var err error
	client, err = api.NewClient(cfg)
	if err != nil {
		log.Fatalf("init consul failed: %v", err)
	}

	// 初始化随机种子
	rand.Seed(time.Now().UnixNano())
}

// GetServiceAddr 获取服务地址，支持负载均衡
func GetServiceAddr(serviceName string) (string, error) {
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil || len(services) == 0 {
		return "", fmt.Errorf("no healthy instance for service: %s", serviceName)
	}

	// 如果只有一个实例，直接返回
	if len(services) == 1 {
		svc := services[0].Service
		return fmt.Sprintf("%s:%d", svc.Address, svc.Port), nil
	}

	// 多个实例时使用轮询负载均衡
	return getServiceAddrWithRoundRobin(serviceName, services)
}

// getServiceAddrWithRoundRobin 轮询负载均衡选择服务实例
func getServiceAddrWithRoundRobin(serviceName string, services []*api.ServiceEntry) (string, error) {
	counterMutex.Lock()
	defer counterMutex.Unlock()

	// 初始化计数器
	if roundRobinCounters[serviceName] == nil {
		counter := int64(0)
		roundRobinCounters[serviceName] = &counter
	}

	// 获取当前计数器值并递增
	counter := roundRobinCounters[serviceName]
	index := int(*counter % int64(len(services)))
	*counter++

	// 选择对应的服务实例
	svc := services[index].Service
	addr := fmt.Sprintf("%s:%d", svc.Address, svc.Port)

	log.Printf("Load balancing: selected service %s instance %d/%d: %s",
		serviceName, index+1, len(services), addr)

	return addr, nil
}

// GetServiceAddrWithRandom 随机负载均衡选择服务实例（备选方案）
func GetServiceAddrWithRandom(serviceName string) (string, error) {
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil || len(services) == 0 {
		return "", fmt.Errorf("no healthy instance for service: %s", serviceName)
	}

	// 随机选择一个实例
	index := rand.Intn(len(services))
	svc := services[index].Service
	addr := fmt.Sprintf("%s:%d", svc.Address, svc.Port)

	log.Printf("Random load balancing: selected service %s instance %d/%d: %s",
		serviceName, index+1, len(services), addr)

	return addr, nil
}

// GetAllServiceInstances 获取服务的所有健康实例信息（用于调试和监控）
func GetAllServiceInstances(serviceName string) ([]string, error) {
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service instances: %v", err)
	}

	var instances []string
	for i, service := range services {
		addr := fmt.Sprintf("%s:%d", service.Service.Address, service.Service.Port)
		instances = append(instances, fmt.Sprintf("Instance %d: %s (ID: %s)",
			i+1, addr, service.Service.ID))
	}

	return instances, nil
}

// GetServiceInstanceCount 获取服务的健康实例数量
func GetServiceInstanceCount(serviceName string) (int, error) {
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get service instances: %v", err)
	}
	return len(services), nil
}
