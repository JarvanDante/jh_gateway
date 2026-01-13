package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

// DomainConfig 域名配置结构
type DomainConfig struct {
	SiteID      int64               `json:"site_id"`
	SiteCode    string              `json:"site_code"`
	Domains     map[string][]string `json:"domains"`
	TotalCount  int                 `json:"total_count"`
	PublishedAt string              `json:"published_at"`
	Version     int64               `json:"version"`
}

// DomainWatcher 域名监听器
type DomainWatcher struct {
	client      *api.Client
	domainMap   map[string]int64         // 域名到site_id的映射
	siteConfigs map[string]*DomainConfig // site_code到配置的映射
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	watchPrefix string
}

var (
	domainWatcher *DomainWatcher
	watcherOnce   sync.Once
)

// InitDomainWatcher 初始化域名监听器
func InitDomainWatcher() error {
	var err error
	watcherOnce.Do(func() {
		if client == nil {
			err = fmt.Errorf("consul client not initialized")
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		domainWatcher = &DomainWatcher{
			client:      client,
			domainMap:   make(map[string]int64),
			siteConfigs: make(map[string]*DomainConfig),
			ctx:         ctx,
			cancel:      cancel,
			watchPrefix: "sites/",
		}

		// 启动监听器
		go domainWatcher.startWatching()

		// 初始加载所有域名配置
		err = domainWatcher.loadAllDomainConfigs()
	})

	return err
}

// startWatching 开始监听Consul KV变化
func (dw *DomainWatcher) startWatching() {
	log.Printf("开始监听域名配置变化，前缀: %s", dw.watchPrefix)

	for {
		select {
		case <-dw.ctx.Done():
			log.Println("域名监听器已停止")
			return
		default:
			if err := dw.watchKVChanges(); err != nil {
				log.Printf("监听KV变化失败: %v，5秒后重试", err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// watchKVChanges 监听KV变化
func (dw *DomainWatcher) watchKVChanges() error {
	queryOpts := &api.QueryOptions{
		WaitTime: 30 * time.Second, // 长轮询30秒
	}

	for {
		select {
		case <-dw.ctx.Done():
			return nil
		default:
			// 获取所有sites/前缀的KV
			pairs, meta, err := dw.client.KV().List(dw.watchPrefix, queryOpts)
			if err != nil {
				return fmt.Errorf("获取KV列表失败: %v", err)
			}

			// 更新查询选项的索引，用于下次长轮询
			queryOpts.WaitIndex = meta.LastIndex

			// 处理KV变化
			dw.processKVChanges(pairs)
		}
	}
}

// processKVChanges 处理KV变化
func (dw *DomainWatcher) processKVChanges(pairs api.KVPairs) {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()

	// 清空现有映射
	dw.domainMap = make(map[string]int64)
	dw.siteConfigs = make(map[string]*DomainConfig)

	for _, pair := range pairs {
		// 只处理domains配置
		if !strings.HasSuffix(pair.Key, "/domains") {
			continue
		}

		// 解析site_code
		parts := strings.Split(pair.Key, "/")
		if len(parts) < 3 {
			continue
		}
		siteCode := parts[1]

		// 解析域名配置
		var config DomainConfig
		if err := json.Unmarshal(pair.Value, &config); err != nil {
			log.Printf("解析域名配置失败，key: %s, error: %v", pair.Key, err)
			continue
		}

		// 存储站点配置
		dw.siteConfigs[siteCode] = &config

		// 构建域名映射
		for _, domains := range config.Domains {
			for _, domain := range domains {
				dw.domainMap[domain] = config.SiteID
			}
		}

		log.Printf("更新站点域名配置: site_code=%s, site_id=%d, 域名数量=%d",
			siteCode, config.SiteID, config.TotalCount)
	}

	log.Printf("域名映射更新完成，共%d个域名映射到%d个站点",
		len(dw.domainMap), len(dw.siteConfigs))
}

// loadAllDomainConfigs 初始加载所有域名配置
func (dw *DomainWatcher) loadAllDomainConfigs() error {
	log.Printf("初始加载域名配置...")

	pairs, _, err := dw.client.KV().List(dw.watchPrefix, nil)
	if err != nil {
		return fmt.Errorf("初始加载域名配置失败: %v", err)
	}

	dw.processKVChanges(pairs)
	return nil
}

// GetSiteIDByDomain 根据域名获取site_id
func GetSiteIDByDomain(domain string) (int64, bool) {
	if domainWatcher == nil {
		log.Printf("域名监听器未初始化")
		return 0, false
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	siteID, exists := domainWatcher.domainMap[domain]
	if !exists {
		log.Printf("域名 %s 未找到对应的site_id", domain)
		return 0, false
	}

	log.Printf("域名 %s 映射到 site_id: %d", domain, siteID)
	return siteID, true
}

// GetSiteConfig 根据site_code获取站点配置
func GetSiteConfig(siteCode string) (*DomainConfig, bool) {
	if domainWatcher == nil {
		return nil, false
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	config, exists := domainWatcher.siteConfigs[siteCode]
	return config, exists
}

// GetAllDomainMappings 获取所有域名映射（用于调试）
func GetAllDomainMappings() map[string]int64 {
	if domainWatcher == nil {
		return make(map[string]int64)
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	// 创建副本避免并发问题
	mappings := make(map[string]int64)
	for domain, siteID := range domainWatcher.domainMap {
		mappings[domain] = siteID
	}

	return mappings
}

// GetDomainStats 获取域名统计信息
func GetDomainStats() map[string]interface{} {
	if domainWatcher == nil {
		return map[string]interface{}{
			"total_domains": 0,
			"total_sites":   0,
			"status":        "not_initialized",
		}
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	return map[string]interface{}{
		"total_domains": len(domainWatcher.domainMap),
		"total_sites":   len(domainWatcher.siteConfigs),
		"status":        "running",
	}
}

// StopDomainWatcher 停止域名监听器
func StopDomainWatcher() {
	if domainWatcher != nil && domainWatcher.cancel != nil {
		domainWatcher.cancel()
		log.Println("域名监听器已停止")
	}
}
