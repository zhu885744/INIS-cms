package facade

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cast"
	"github.com/unti-io/go-utils/utils"
)

type H map[string]any

const (
	// ConfigPath - 配置目录
	ConfigPath = "config"
	// ModeToml - 配置文件格式
	ModeToml = "toml"
	// ConfigNameApp - 应用配置文件名
	ConfigNameApp = "app"
)

// AppToml - APP配置文件
var AppToml *utils.ViperResponse

// initAppToml - 初始化App配置文件
func initAppToml() {
	item := utils.Viper(utils.ViperModel{
		Path:    ConfigPath,
		Mode:    ModeToml,
		Name:    ConfigNameApp,
		Content: appTomlContent(),
	}).Read()

	if item.Error != nil {
		Log.Error(map[string]any{
			"error":     item.Error,
			"func_name": utils.Caller().FuncName,
			"file_name": utils.Caller().FileName,
			"file_line": utils.Caller().Line},
			"App配置初始化错误")
	}

	AppToml = &item
}

// init - 初始化函数
func init() {
	// 最先执行：config 目录可能整份不存在（只发一个二进制的部署方式），
	// 这里先把目录与缺失的配置文件补齐，见 bootstrap.go
	ensureBootstrap()

	// 本文件是包内最早执行的 init（文件名字典序），而 log.go 的初始化排在其后；
	// 这里在配置异常时会输出日志，故先确保日志组件就绪
	ensureLogReady()

	initAppToml()
	initApp()

	WatchConfigChange(AppToml, initApp)
}

// initApp - 初始化应用
func initApp() {

}

// SocketEnabled - 实时连接总开关（config/app.toml 的 [socket] enabled）
//
// ⚠️ 不能写成 AppToml.Get("socket.enabled", true)：
// utils.ViperResponse.Get 的实现是 `Ternary(!Is.Empty(result), result, item)`，
// 而 Is.Empty 把 bool 的 false 当成「空值」—— 配置里写 enabled = false 时会命中
// 默认值 true，开关永远关不掉（debug / enable_origin_check 等 bool 项同样受影响）。
// 这里直接读底层 viper，用 nil 判断「配置项是否存在」，false 能原样返回。
func SocketEnabled() bool {
	if AppToml == nil || AppToml.Viper == nil {
		return true
	}
	value := AppToml.Viper.Get("socket.enabled")
	if value == nil {
		return true
	}
	return cast.ToBool(value)
}

// WatchConfigChange - 监听配置文件变化（导出函数，供包内其他文件使用）
func WatchConfigChange(config *utils.ViperResponse, callback func()) {
	if config == nil || config.Viper == nil {
		return
	}
	config.Viper.WatchConfig()
	config.Viper.OnConfigChange(func(event fsnotify.Event) {
		callback()
	})
}
