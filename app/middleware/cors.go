package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"inis/app/facade"
)

// CORS常量配置
const (
	defaultCorsMaxAge                = 1800
	defaultCorsAllowCredentials      = true
	defaultCorsAllowedMethods        = "GET, POST, PATCH, PUT, DELETE, OPTIONS"
	defaultCorsAllowedHeaders        = "X-Khronos, X-Gorgon, X-Argus, X-Ss-Stub, Token, Authorization, i-api-key, Content-Type, If-Match, If-Modified-Since, If-None-Match, If-Unmodified-Since, X-CSRF-TOKEN, X-Requested-With"
	defaultCorsExposedHeaders        = "Content-Type, Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers"
	defaultCorsDefaultOrigin         = "http://localhost:8642"
	defaultCorsAllowedOriginsDefault = "http://localhost:3000,http://127.0.0.1:3000"
)

// corsValue - 读取跨域配置项（环境变量优先，其次 app.toml，最后默认值）
//
// 不能写成 facade.AppToml.Get("system.cors.enabled", true).(bool)：
// utils.ViperResponse.Get 内部用 `!Is.Empty(result)` 判断「有没有读到」，而 Is.Empty
// 把 bool 的 false 当成空值 —— 配置写 enabled = false 时会回退成默认 true，
// 开关永远关不掉（[socket] enabled 也是同一个坑）。
// 这里直接读底层 viper，用 nil 判断「配置项是否存在」。
func corsValue(key string, def any) any {
	if facade.AppToml == nil || facade.AppToml.Viper == nil {
		return def
	}
	if value := facade.AppToml.Viper.Get("system.cors." + key); value != nil {
		return value
	}
	return def
}

// Cors - 跨域中间件
func Cors() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.Writer.Header()

		enabled := cast.ToBool(corsValue("enabled", true))
		if !enabled {
			ctx.Next()
			return
		}

		allowedOriginsStr := cast.ToString(corsValue("allowed_origins", defaultCorsAllowedOriginsDefault))
		allowedOrigins := make(map[string]bool)
		for _, origin := range strings.Split(allowedOriginsStr, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins[origin] = true
			}
		}

		maxAge := cast.ToInt(corsValue("max_age", defaultCorsMaxAge))
		allowCredentials := cast.ToBool(corsValue("allow_credentials", defaultCorsAllowCredentials))
		allowedMethods := cast.ToString(corsValue("allowed_methods", defaultCorsAllowedMethods))
		allowedHeaders := cast.ToString(corsValue("allowed_headers", defaultCorsAllowedHeaders))
		exposedHeaders := cast.ToString(corsValue("exposed_headers", defaultCorsExposedHeaders))
		defaultOrigin := cast.ToString(corsValue("default_origin", defaultCorsDefaultOrigin))

		header.Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
		header.Set("Access-Control-Allow-Credentials", strconv.FormatBool(allowCredentials))
		// 不要在这里写死 Content-Type。
		// 原实现在全局中间件里设成 application/json，会连带静态资源、首页 HTML、
		// WebSocket 握手一起被标成 JSON —— 浏览器对 CSS / JS 做严格 MIME 校验，
		// 类型不对会直接拒载。Content-Type 由各 handler（ctx.JSON / ServeFile / ctx.Data）自己给。
		header.Set("Access-Control-Allow-Methods", strings.TrimSpace(allowedMethods))
		header.Set("Access-Control-Allow-Headers", strings.TrimSpace(allowedHeaders))
		header.Set("Access-Control-Expose-Headers", strings.TrimSpace(exposedHeaders))

		origin := ctx.Request.Header.Get("Origin")
		// 仅当来源在允许列表中时才回显该来源；否则不设置跨域头（浏览器将阻止跨域读取）
		if origin != "" {
			if allowedOrigins[origin] {
				header.Set("Access-Control-Allow-Origin", origin)
				// 非通配符来源时，允许携带凭证
				header.Set("Vary", "Origin")
			} else {
				// 非白名单来源：不放行跨域，OPTIONS 预检直接拒绝
				if strings.ToUpper(ctx.Request.Method) == "OPTIONS" {
					ctx.AbortWithStatus(http.StatusForbidden)
					return
				}
				ctx.Next()
				return
			}
		} else {
			// 同源/非跨域请求（无 Origin 头），回退到默认来源
			header.Set("Access-Control-Allow-Origin", defaultOrigin)
		}

		if strings.ToUpper(ctx.Request.Method) == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
