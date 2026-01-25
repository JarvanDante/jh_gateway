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

// SiteConfig 站点完整配置结构
type SiteConfig struct {
	Meta   MetaConfig   `json:"Meta"`
	Base   BaseConfig   `json:"Base"`
	Domain DomainConfig `json:"Domain"`
}

// MetaConfig 元数据配置
type MetaConfig struct {
	Version     int64  `json:"version"`
	PublishedAt string `json:"published_at"`
	AdminId     int    `json:"admin_id"`
}

// BaseConfig 基础配置
type BaseConfig struct {
	SiteId        int    `json:"site_id"`
	Code          string `json:"code"`
	TimeZone      string `json:"timezone"`
	Currency      string `json:"currency"`
	DbLinkSite    string `json:"db_link_site"`
	DbLinkBalance string `json:"db_link_balance"`
	DbLinkGame    string `json:"db_link_game"`
	Status        int    `json:"status"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	API      []string `json:"api"`
	H5       []string `json:"h5"`
	Admin    []string `json:"admin"`
	Callback []string `json:"callback"`
}

// DomainWatcher 域名监听器
type DomainWatcher struct {
	client      *api.Client
	domainMap   map[string]*SiteInfo   // 域名到站点信息的映射
	siteConfigs map[string]*SiteConfig // site_code到配置的映射
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	watchPrefix string
}

// SiteInfo 站点信息
type SiteInfo struct {
	SiteID   int64
	SiteCode string
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
			domainMap:   make(map[string]*SiteInfo),
			siteConfigs: make(map[string]*SiteConfig),
			ctx:         ctx,
			cancel:      cancel,
			watchPrefix: "sites/",
		}

		// 初始加载所有域名配置（带重试机制）
		maxRetries := 5
		retryDelay := 3 * time.Second

		for i := 0; i < maxRetries; i++ {
			err = domainWatcher.loadAllDomainConfigs()

			// 检查是否加载成功且有数据
			domainCount := len(domainWatcher.domainMap)

			if err == nil && domainCount > 0 {
				log.Printf("✅ 域名配置加载成功，共加载 %d 个域名映射", domainCount)
				break
			}

			// 如果是最后一次尝试，记录错误并继续（允许服务启动，但会警告）
			if i == maxRetries-1 {
				log.Printf("⚠️  警告：经过 %d 次尝试后仍未加载到域名配置", maxRetries)
				log.Printf("⚠️  Gateway将继续启动，但域名解析可能失败")
				log.Printf("⚠️  请检查Consul服务状态和KV配置: sites/")
				break
			}

			// 记录重试信息
			if err != nil {
				log.Printf("❌ 加载域名配置失败（尝试 %d/%d）: %v", i+1, maxRetries, err)
			} else {
				log.Printf("⚠️  域名配置为空（尝试 %d/%d），可能Consul数据尚未就绪", i+1, maxRetries)
			}

			log.Printf("⏳ %v 后重试...", retryDelay)
			time.Sleep(retryDelay)
		}

		// 启动监听器
		go domainWatcher.startWatching()
	})

	return err
}

// startWatching 开始监听Consul KV变化
func (dw *DomainWatcher) startWatching() {
	log.Printf("开始监听域名配置变化，前缀: %s", dw.watchPrefix)

	queryOpts := &api.QueryOptions{
		WaitTime: 30 * time.Second,
	}

	for {
		select {
		case <-dw.ctx.Done():
			log.Println("域名监听器已停止")
			return
		default:
			pairs, meta, err := dw.client.KV().List(dw.watchPrefix, queryOpts)
			if err != nil {
				log.Printf("监听KV变化失败: %v，5秒后重试", err)
				time.Sleep(5 * time.Second)
				continue
			}

			queryOpts.WaitIndex = meta.LastIndex
			dw.processKVChanges(pairs)
		}
	}
}

// processKVChanges 处理KV变化
func (dw *DomainWatcher) processKVChanges(pairs api.KVPairs) {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()

	newDomainMap := make(map[string]*SiteInfo)
	newSiteConfigs := make(map[string]*SiteConfig)

	for _, pair := range pairs {
		// 解析 key: sites/{site_code}
		parts := strings.Split(pair.Key, "/")
		if len(parts) != 2 {
			continue
		}
		siteCode := parts[1]

		// 解析站点配置
		var config SiteConfig
		if err := json.Unmarshal(pair.Value, &config); err != nil {
			log.Printf("解析站点配置失败，key: %s, error: %v", pair.Key, err)
			continue
		}

		// 存储站点配置
		newSiteConfigs[siteCode] = &config

		// 构建域名映射
		siteInfo := &SiteInfo{
			SiteID:   int64(config.Base.SiteId),
			SiteCode: config.Base.Code,
		}

		// 映射所有类型的域名
		for _, domain := range config.Domain.API {
			newDomainMap[domain] = siteInfo
		}
		for _, domain := range config.Domain.H5 {
			newDomainMap[domain] = siteInfo
		}
		for _, domain := range config.Domain.Admin {
			newDomainMap[domain] = siteInfo
		}
		for _, domain := range config.Domain.Callback {
			newDomainMap[domain] = siteInfo
		}

		totalDomains := len(config.Domain.API) + len(config.Domain.H5) +
			len(config.Domain.Admin) + len(config.Domain.Callback)

		// log.Printf("更新站点配置: site_code=%s, site_id=%d, 域名数量=%d",
		// 	siteCode, config.Base.SiteId, totalDomains)
		_ = totalDomains // 避免未使用变量警告
	}

	dw.domainMap = newDomainMap
	dw.siteConfigs = newSiteConfigs

	// log.Printf("域名映射更新完成，共%d个域名映射到%d个站点",
	// 	len(dw.domainMap), len(dw.siteConfigs))
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

	siteInfo, exists := domainWatcher.domainMap[domain]
	if !exists {
		log.Printf("域名 %s 未找到对应的站点", domain)
		return 0, false
	}

	log.Printf("域名 %s 映射到 site_id: %d, site_code: %s", domain, siteInfo.SiteID, siteInfo.SiteCode)
	return siteInfo.SiteID, true
}

// GetSiteCodeByDomain 根据域名获取site_code
func GetSiteCodeByDomain(domain string) (string, bool) {
	if domainWatcher == nil {
		log.Printf("域名监听器未初始化")
		return "", false
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	siteInfo, exists := domainWatcher.domainMap[domain]
	if !exists {
		log.Printf("域名 %s 未找到对应的站点", domain)
		return "", false
	}

	return siteInfo.SiteCode, true
}

// GetSiteInfoByDomain 根据域名获取完整站点信息
func GetSiteInfoByDomain(domain string) (*SiteInfo, bool) {
	if domainWatcher == nil {
		log.Printf("域名监听器未初始化")
		return nil, false
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	siteInfo, exists := domainWatcher.domainMap[domain]
	if !exists {
		return nil, false
	}

	return siteInfo, true
}

// GetSiteConfig 根据site_code获取站点配置
func GetSiteConfig(siteCode string) (*SiteConfig, bool) {
	if domainWatcher == nil {
		return nil, false
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	config, exists := domainWatcher.siteConfigs[siteCode]
	return config, exists
}

// GetAllDomainMappings 获取所有域名映射（用于调试）
func GetAllDomainMappings() map[string]*SiteInfo {
	if domainWatcher == nil {
		return make(map[string]*SiteInfo)
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	mappings := make(map[string]*SiteInfo)
	for domain, siteInfo := range domainWatcher.domainMap {
		mappings[domain] = siteInfo
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

// GetDomainCount 获取当前加载的域名数量
func GetDomainCount() int {
	if domainWatcher == nil {
		return 0
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	return len(domainWatcher.domainMap)
}

// GetSiteCount 获取当前加载的站点数量
func GetSiteCount() int {
	if domainWatcher == nil {
		return 0
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	return len(domainWatcher.siteConfigs)
}

// GetHealthStatus 获取域名监听器的健康状态
func GetHealthStatus() map[string]interface{} {
	if domainWatcher == nil {
		return map[string]interface{}{
			"status":       "uninitialized",
			"domain_count": 0,
			"site_count":   0,
		}
	}

	domainWatcher.mutex.RLock()
	defer domainWatcher.mutex.RUnlock()

	status := "healthy"
	if len(domainWatcher.domainMap) == 0 {
		status = "warning"
	}

	return map[string]interface{}{
		"status":       status,
		"domain_count": len(domainWatcher.domainMap),
		"site_count":   len(domainWatcher.siteConfigs),
	}
}
