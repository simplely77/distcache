package distcache

import (
	"sync"
	"time"

	"github.com/simplely77/distcache/bloomfilter"
	"github.com/simplely77/distcache/countminsketch"
)

type HotKeyDetector struct {
	bf        *bloomfilter.BloomFilter
	cms       *countminsketch.CountMinSketch
	hotKeys   sync.Map // key -> ByteView
	threshold uint64
	decayIntv time.Duration
	stopCh    chan struct{} // 用于停止定期衰减
}

func NewHotKeyDetector(threshold uint64, decayInterval time.Duration) *HotKeyDetector {
	h := &HotKeyDetector{
		bf:        bloomfilter.NewBloomFilter(1_000_000, 5),
		cms:       countminsketch.NewCountMinSketch(0.001, 0.99),
		threshold: threshold,
		decayIntv: decayInterval,
		stopCh:    make(chan struct{}),
	}
	go h.periodicDecay()
	return h
}

// RecordKey 在访问时调用
func (h *HotKeyDetector) RecordKey(key string, value ByteView) {
	if !h.bf.Test(key) {
		h.bf.Add(key)
		if IsMetricsEnabled() {
			GetMetrics().RecordBloomFilter("miss")
		}
		return
	}

	if IsMetricsEnabled() {
		GetMetrics().RecordBloomFilter("hit")
	}

	h.cms.Add(key, 1)
	count := h.cms.Count(key)
	if count >= h.threshold {
		// 检查是否是新晋升的热点key
		if _, exists := h.hotKeys.Load(key); !exists {
			if IsMetricsEnabled() {
				GetMetrics().RecordHotKey("promoted")
			}
		}
		h.hotKeys.Store(key, value)
	}
}

// 获取热点key
func (h *HotKeyDetector) GetHot(key string) (ByteView, bool) {
	v, ok := h.hotKeys.Load(key)
	if !ok {
		return ByteView{}, false
	}
	return v.(ByteView), true
}

// 定期衰减频率
func (h *HotKeyDetector) periodicDecay() {
	ticker := time.NewTicker(h.decayIntv)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.cms.Decay()
			// 可选：检查热点key，如果访问下降，删除
			h.hotKeys.Range(func(k, _ interface{}) bool {
				key := k.(string)
				if h.cms.Count(key) < h.threshold/2 {
					h.hotKeys.Delete(key)
					if IsMetricsEnabled() {
						GetMetrics().RecordHotKey("demoted")
					}
				}
				return true
			})
		case <-h.stopCh:
			return
		}
	}
}

// Stop 停止热点检测器
func (h *HotKeyDetector) Stop() {
	close(h.stopCh)
}
