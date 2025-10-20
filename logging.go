package distcache

import "sync/atomic"

// 全局日志控制
var (
	// enableLogging 控制是否启用日志输出（默认关闭以提升性能）
	enableLogging int32 = 0
)

// EnableLogging 启用日志输出（调试时使用，会降低性能）
func EnableLogging() {
	atomic.StoreInt32(&enableLogging, 1)
}

// DisableLogging 禁用日志输出（默认状态，性能最优）
func DisableLogging() {
	atomic.StoreInt32(&enableLogging, 0)
}

// IsLoggingEnabled 检查是否启用日志
func IsLoggingEnabled() bool {
	return atomic.LoadInt32(&enableLogging) == 1
}
