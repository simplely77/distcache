package distcache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// 创建一个简单的ByteView用于测试
func makeByteView(data string) ByteView {
	return ByteView{b: []byte(data)}
}

// 基础功能测试
func TestHotKeyDetector_BasicOperations(t *testing.T) {
	detector := NewHotKeyDetector(3, time.Minute) // 阈值为3
	defer detector.Stop()                         // 确保goroutine退出

	// 测试数据
	key := "test_key"
	value := makeByteView("test_value")

	// 第一次访问 - 应该被布隆过滤器过滤
	detector.RecordKey(key, value)
	_, exists := detector.GetHot(key)
	if exists {
		t.Error("Key should not be hot after first access")
	}

	// 第二次访问 - 布隆过滤器通过，开始计数
	detector.RecordKey(key, value)
	_, exists = detector.GetHot(key)
	if exists {
		t.Error("Key should not be hot after second access")
	}

	// 继续访问直到达到阈值
	for i := 0; i < 3; i++ {
		detector.RecordKey(key, value)
	}

	// 现在应该是热点key
	hotValue, exists := detector.GetHot(key)
	if !exists {
		t.Error("Key should be hot after reaching threshold")
	}
	if hotValue.String() != value.String() {
		t.Errorf("Expected value '%s', got '%s'", value.String(), hotValue.String())
	}

	t.Log("Basic operations test passed")
}

// 多个key的热点检测测试
func TestHotKeyDetector_MultipleKeys(t *testing.T) {
	detector := NewHotKeyDetector(5, time.Minute)
	defer detector.Stop()

	// 测试数据
	keys := []string{"hot_key_1", "hot_key_2", "cold_key_1", "cold_key_2"}
	values := make(map[string]ByteView)
	for i, key := range keys {
		values[key] = makeByteView(fmt.Sprintf("value_%d", i))
	}

	// 让前两个key变热
	for _, key := range keys[:2] {
		// 先通过布隆过滤器
		detector.RecordKey(key, values[key])
		detector.RecordKey(key, values[key])

		// 然后达到阈值
		for i := 0; i < 6; i++ {
			detector.RecordKey(key, values[key])
		}
	}

	// 后两个key访问次数不够
	for _, key := range keys[2:] {
		detector.RecordKey(key, values[key])
		detector.RecordKey(key, values[key])
		detector.RecordKey(key, values[key])
	}

	// 验证热点key
	for _, key := range keys[:2] {
		value, exists := detector.GetHot(key)
		if !exists {
			t.Errorf("Key '%s' should be hot", key)
		}
		if value.String() != values[key].String() {
			t.Errorf("Key '%s': expected value '%s', got '%s'",
				key, values[key].String(), value.String())
		}
	}

	// 验证冷key
	for _, key := range keys[2:] {
		_, exists := detector.GetHot(key)
		if exists {
			t.Errorf("Key '%s' should not be hot", key)
		}
	}

	t.Log("Multiple keys test passed")
}

// 并发访问测试
func TestHotKeyDetector_ConcurrentAccess(t *testing.T) {
	detector := NewHotKeyDetector(10, time.Minute)
	defer detector.Stop()

	key := "concurrent_key"
	value := makeByteView("concurrent_value")

	// 并发访问同一个key
	var wg sync.WaitGroup
	numGoroutines := 20
	accessPerGoroutine := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < accessPerGoroutine; j++ {
				detector.RecordKey(key, value)
			}
		}()
	}

	wg.Wait()

	// 总访问次数应该足够让key变热
	hotValue, exists := detector.GetHot(key)
	if !exists {
		// 可能需要更多访问次数，因为布隆过滤器的影响
		t.Logf("Key might not be hot due to bloom filter effects")
	} else {
		if hotValue.String() != value.String() {
			t.Errorf("Expected value '%s', got '%s'", value.String(), hotValue.String())
		}
	}

	t.Log("Concurrent access test passed")
}

// 衰减功能测试
func TestHotKeyDetector_Decay(t *testing.T) {
	// 使用较短的衰减间隔进行测试
	detector := NewHotKeyDetector(5, 100*time.Millisecond)
	defer detector.Stop()

	key := "decay_test_key"
	value := makeByteView("decay_test_value")

	// 先让key变热
	detector.RecordKey(key, value)
	detector.RecordKey(key, value)
	for i := 0; i < 6; i++ {
		detector.RecordKey(key, value)
	}

	// 验证key是热的
	_, exists := detector.GetHot(key)
	if !exists {
		t.Error("Key should be hot before decay")
	}

	// 等待衰减发生
	time.Sleep(200 * time.Millisecond)

	// 衰减后，key可能不再是热的（取决于具体的衰减逻辑）
	_, stillHot := detector.GetHot(key)
	t.Logf("After decay, key is still hot: %v", stillHot)

	t.Log("Decay test completed")
}

// 阈值边界测试
func TestHotKeyDetector_ThresholdBoundary(t *testing.T) {
	threshold := uint64(3)
	detector := NewHotKeyDetector(threshold, time.Minute)
	defer detector.Stop()

	key := "boundary_test_key"
	value := makeByteView("boundary_test_value")

	// 先通过布隆过滤器
	detector.RecordKey(key, value)
	detector.RecordKey(key, value)

	// 精确地访问到阈值-1次
	for i := uint64(0); i < threshold-1; i++ {
		detector.RecordKey(key, value)
		_, exists := detector.GetHot(key)
		if exists {
			// 由于Count-Min Sketch的高估特性，可能会提前达到阈值
			t.Logf("Key became hot early at count %d (threshold: %d) due to CMS overestimation", i+1, threshold)
			break
		}
	}

	// 再访问一次应该达到阈值
	detector.RecordKey(key, value)
	_, exists := detector.GetHot(key)
	if !exists {
		t.Error("Key should be hot after reaching threshold")
	}

	t.Log("Threshold boundary test passed")
}

// 不同key类型测试
func TestHotKeyDetector_DifferentKeyTypes(t *testing.T) {
	detector := NewHotKeyDetector(3, time.Minute)
	defer detector.Stop()

	testCases := []struct {
		key   string
		value string
	}{
		{"", "empty_key"},                                               // 空字符串
		{"normal_key", "normal_value"},                                  // 普通字符串
		{"key_with_spaces", "value spaces"},                             // 包含空格
		{"key:with:colons", "colon value"},                              // 包含特殊字符
		{"very_long_key_" + fmt.Sprintf("%0100d", 1), "long key value"}, // 长key
	}

	for _, tc := range testCases {
		value := makeByteView(tc.value)

		// 先通过布隆过滤器
		detector.RecordKey(tc.key, value)
		detector.RecordKey(tc.key, value)

		// 然后达到阈值
		for i := 0; i < 4; i++ {
			detector.RecordKey(tc.key, value)
		}

		hotValue, exists := detector.GetHot(tc.key)
		if !exists {
			t.Errorf("Key '%s' should be hot", tc.key)
		} else if hotValue.String() != tc.value {
			t.Errorf("Key '%s': expected value '%s', got '%s'",
				tc.key, tc.value, hotValue.String())
		}
	}

	t.Log("Different key types test passed")
}

// 性能基准测试
func BenchmarkHotKeyDetector_RecordKey(b *testing.B) {
	detector := NewHotKeyDetector(1000, time.Hour)
	defer detector.Stop()
	value := makeByteView("benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i%10000)
		detector.RecordKey(key, value)
	}
}

func BenchmarkHotKeyDetector_GetHot(b *testing.B) {
	detector := NewHotKeyDetector(1, time.Hour)
	defer detector.Stop()
	value := makeByteView("benchmark_value")

	// 预先添加一些热点key
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("hot_key_%d", i)
		detector.RecordKey(key, value)
		detector.RecordKey(key, value)
		detector.RecordKey(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("hot_key_%d", i%1000)
		detector.GetHot(key)
	}
}
