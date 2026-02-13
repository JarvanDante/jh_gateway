package middleware

import (
	"context"
	"log"
	"strconv"
	"strings"

	"jh_gateway/internal/registry"

	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc/metadata"
)

// DomainResolver 域名解析中间件
func DomainResolver(r *ghttp.Request) {
	// 获取请求的Host头
	host := r.Header.Get("Host")
	if host == "" {
		host = r.Host
	}

	// 移除端口号（如果有）
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	log.Printf("请求域名: %s", host)

	// 将域名设置到请求上下文中
	r.SetCtxVar("domain", host)

	// 根据域名获取站点信息
	siteInfo, exists := registry.GetSiteInfoByDomain(host)
	if exists {
		// 将site_id和site_code设置到请求上下文中
		r.SetCtxVar("site_id", siteInfo.SiteID)
		r.SetCtxVar("site_code", siteInfo.SiteCode)

		// 从KV获取站点配置，提取currency
		if siteConfig, configExists := registry.GetSiteConfig(siteInfo.SiteCode); configExists {
			r.SetCtxVar("currency", siteConfig.Base.Currency)
			log.Printf("✓ 域名 %s 解析成功 -> site_id: %d, site_code: %s, currency: %s",
				host, siteInfo.SiteID, siteInfo.SiteCode, siteConfig.Base.Currency)
		} else {
			log.Printf("⚠ 域名 %s 解析到 site_id: %d, site_code: %s (未找到KV配置)",
				host, siteInfo.SiteID, siteInfo.SiteCode)
		}
	} else {
		// 域名未命中时，支持通过请求头/参数显式指定站点（用于本地开发或反向代理场景）
		if fallbackSiteID, fallbackSiteCode, fallbackCurrency, ok := resolveSiteFromRequest(r); ok {
			r.SetCtxVar("site_id", fallbackSiteID)
			r.SetCtxVar("site_code", fallbackSiteCode)
			r.SetCtxVar("currency", fallbackCurrency)
			log.Printf("⚠ 域名 %s 未命中，使用显式站点: site_id=%d, site_code=%s, currency=%s",
				host, fallbackSiteID, fallbackSiteCode, fallbackCurrency)
		} else {
			// 如果没有找到对应的站点，设置默认值
			log.Printf("✗ 警告: 域名 %s 未找到对应的站点，使用默认值", host)
			r.SetCtxVar("site_id", int64(0))
			r.SetCtxVar("site_code", "")
			r.SetCtxVar("currency", "")
		}
	}

	// 继续处理请求
	r.Middleware.Next()
}

// resolveSiteFromRequest 当域名无法解析时，从请求头/参数兜底解析站点
func resolveSiteFromRequest(r *ghttp.Request) (siteID int64, siteCode, currency string, ok bool) {
	// 优先使用 site_code 定位站点（最可靠）
	siteCode = strings.TrimSpace(r.Header.Get("X-Site-Code"))
	if siteCode == "" {
		siteCode = strings.TrimSpace(r.Get("site_code").String())
	}
	if siteCode != "" {
		if siteConfig, exists := registry.GetSiteConfig(siteCode); exists && siteConfig != nil {
			resolvedCode := strings.TrimSpace(siteConfig.Base.Code)
			if resolvedCode == "" {
				resolvedCode = siteCode
			}
			return int64(siteConfig.Base.SiteId), resolvedCode, strings.TrimSpace(siteConfig.Base.Currency), true
		}
	}

	// 兜底使用 site_id
	siteIDStr := strings.TrimSpace(r.Header.Get("X-Site-Id"))
	if siteIDStr != "" {
		if id, err := strconv.ParseInt(siteIDStr, 10, 64); err == nil && id > 0 {
			return id, siteCode, "", true
		}
	}
	if id := r.Get("site_id").Int64(); id > 0 {
		return id, siteCode, "", true
	}

	return 0, "", "", false
}

// AddSiteIDToGRPCContext 将site_id、site_code、currency和domain添加到gRPC上下文中
func AddSiteIDToGRPCContext(ctx context.Context, r *ghttp.Request) context.Context {
	// 从请求上下文中获取site_id、site_code、currency和domain
	siteIdVar := r.GetCtxVar("site_id")
	siteCodeVar := r.GetCtxVar("site_code")
	currencyVar := r.GetCtxVar("currency")
	domainVar := r.GetCtxVar("domain")

	if siteIdVar != nil && siteIdVar.Int64() > 0 {
		// 创建gRPC metadata
		md := metadata.New(map[string]string{
			"site_id": siteIdVar.String(),
		})

		// 添加site_code
		if siteCodeVar != nil && siteCodeVar.String() != "" {
			md.Set("site_code", siteCodeVar.String())
		}

		// 添加currency
		if currencyVar != nil && currencyVar.String() != "" {
			md.Set("currency", currencyVar.String())
		}

		// 添加domain
		if domainVar != nil && domainVar.String() != "" {
			md.Set("domain", domainVar.String())
		}

		// 将metadata添加到现有的outgoing context中
		if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
			// 合并现有的metadata
			for key, values := range existingMD {
				for _, value := range values {
					md.Append(key, value)
				}
			}
		}

		ctx = metadata.NewOutgoingContext(ctx, md)
		log.Printf("已将 site_id: %d, site_code: %s, currency: %s, domain: %s 添加到 gRPC metadata",
			siteIdVar.Int64(), siteCodeVar.String(), currencyVar.String(), domainVar.String())
	}

	return ctx
}

// GetSiteIDFromRequest 从请求中获取site_id
func GetSiteIDFromRequest(r *ghttp.Request) int64 {
	if siteIdVar := r.GetCtxVar("site_id"); siteIdVar != nil {
		return siteIdVar.Int64()
	}
	return 0
}

// GetSiteCodeFromRequest 从请求中获取site_code
func GetSiteCodeFromRequest(r *ghttp.Request) string {
	if siteCodeVar := r.GetCtxVar("site_code"); siteCodeVar != nil {
		return siteCodeVar.String()
	}
	return ""
}

// GetCurrencyFromRequest 从请求中获取currency
func GetCurrencyFromRequest(r *ghttp.Request) string {
	if currencyVar := r.GetCtxVar("currency"); currencyVar != nil {
		return currencyVar.String()
	}
	return ""
}

// GetDomainFromRequest 从请求中获取domain
func GetDomainFromRequest(r *ghttp.Request) string {
	if domainVar := r.GetCtxVar("domain"); domainVar != nil {
		return domainVar.String()
	}
	return ""
}

// GetSiteIDFromContext 从上下文中获取site_id（用于gRPC服务端）
func GetSiteIDFromContext(ctx context.Context) int64 {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if siteIDs := md.Get("site_id"); len(siteIDs) > 0 {
			if siteID := strings.TrimSpace(siteIDs[0]); siteID != "" {
				// 转换为int64
				if id, err := strconv.ParseInt(siteID, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// GetSiteCodeFromContext 从上下文中获取site_code（用于gRPC服务端）
func GetSiteCodeFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if siteCodes := md.Get("site_code"); len(siteCodes) > 0 {
			return strings.TrimSpace(siteCodes[0])
		}
	}
	return ""
}

// GetCurrencyFromContext 从上下文中获取currency（用于gRPC服务端）
func GetCurrencyFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if currencies := md.Get("currency"); len(currencies) > 0 {
			return strings.TrimSpace(currencies[0])
		}
	}
	return ""
}

// GetDomainFromContext 从上下文中获取domain（用于gRPC服务端）
func GetDomainFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if domains := md.Get("domain"); len(domains) > 0 {
			return strings.TrimSpace(domains[0])
		}
	}
	return ""
}
