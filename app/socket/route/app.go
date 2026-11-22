package route

import (
	"fmt"

	apimiddleware "inis/app/api/middleware"
	"inis/app/facade"
	"inis/app/socket/controller"
	socketmiddleware "inis/app/socket/middleware"

	"github.com/gin-gonic/gin"
)

func Route(engine *gin.Engine) {
	// 总开关在 Connect 处理函数里判断（见 controller.Index.Connect）：
	// 那里每次连接读一次配置，改 app.toml 后热更新即时生效，不必重启；
	// 若放在这里，就只在启动/重建路由时读一次，容易出现「改了配置没反应」。
	fmt.Printf("[socket] 实时连接总开关：[socket] enabled = %v\n", facade.SocketEnabled())

	// IpBlack 必须在 Auth 之前：WebSocket 握手同样要受 IP 封禁约束，
	// 否则被封禁的 IP 只要改走 /socket 就能继续连（此前 /socket 完全没有
	// 走 api 分组的任何风控中间件）。
	socket := engine.Group("/socket", apimiddleware.IpBlack(), apimiddleware.Auth(), socketmiddleware.App)
	{
		class := controller.Index{}
		socket.GET("", class.Connect)
		socket.GET("/", class.Connect)
	}
}
