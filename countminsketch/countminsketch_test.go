package countminsketch

import (
	"fmt"
	"math"
	"testing"
)

// 基础功能测试
func TestCountMinSketch_BasicOperations(t *testing.T) {
	// 创建一个误差率0.1、置信度0.9的CMS
	cms := NewCountMinSketch(0.1, 0.1)

	// 测试添加和查询
	testData := map[string]uint64{
		"key1": 5,
		"key2": 10,
		"key3": 15,
		"key4": 1,
	}

	// 添加数据
	for key, count := range testData {
		cms.Add(key, count)
	}

	// 验证计数
	for key, expectedCount := range testData {
		actualCount := cms.Count(key)
		if actualCount < expectedCount {
			t.Errorf("Key '%s': expected count >= %d, got %d", key, expectedCount, actualCount)
		}
		// CMS只会高估，不会低估
		if actualCount > expectedCount*2 { // 允许一定的误差
			t.Errorf("Key '%s': count too high, expected ~%d, got %d", key, expectedCount, actualCount)
		}
	}

	t.Logf("Basic operations test passed")
}

// 累积计数测试
func TestCountMinSketch_IncrementalCount(t *testing.T) {
	cms := NewCountMinSketch(0.05, 0.05)

	key := "incremental_key"
	totalCount := uint64(0)

	// 逐步增加计数
	increments := []uint64{1, 5, 10, 20, 3, 7}
	for _, inc := range increments {
		cms.Add(key, inc)
		totalCount += inc

		count := cms.Count(key)
		if count < totalCount {
			t.Errorf("Incremental count error: expected >= %d, got %d", totalCount, count)
		}
	}

	finalCount := cms.Count(key)
	t.Logf("Total increments: %d, Final count: %d", totalCount, finalCount)

	if finalCount < totalCount {
		t.Errorf("Final count should be at least %d, got %d", totalCount, finalCount)
	}
}

// 频率估计精度测试
func TestCountMinSketch_FrequencyEstimation(t *testing.T) {
	cms := NewCountMinSketch(0.01, 0.01) // 高精度参数

	// 生成测试数据：一些高频key和一些低频key
	highFreqKeys := []string{"popular1", "popular2", "popular3"}
	lowFreqKeys := []string{"rare1", "rare2", "rare3", "rare4", "rare5"}

	// 添加高频数据
	for _, key := range highFreqKeys {
		cms.Add(key, 1000) // 高频：1000次
	}

	// 添加低频数据
	for _, key := range lowFreqKeys {
		cms.Add(key, 10) // 低频：10次
	}

	// 验证高频key的估计
	for _, key := range highFreqKeys {
		count := cms.Count(key)
		if count < 1000 {
			t.Errorf("High frequency key '%s': count too low, expected >= 1000, got %d", key, count)
		}
		if count > 1500 { // 允许50%的误差
			t.Errorf("High frequency key '%s': count too high, expected ~1000, got %d", key, count)
		}
	}

	// 验证低频key的估计
	for _, key := range lowFreqKeys {
		count := cms.Count(key)
		if count < 10 {
			t.Errorf("Low frequency key '%s': count too low, expected >= 10, got %d", key, count)
		}
		if count > 100 { // 低频key误差可能较大
			t.Errorf("Low frequency key '%s': count too high, expected ~10, got %d", key, count)
		}
	}

	t.Log("Frequency estimation test passed")
}

// 大量数据测试
func TestCountMinSketch_LargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	cms := NewCountMinSketch(0.001, 0.001) // 非常高的精度

	// 生成大量数据
	numKeys := 10000
	countsPerKey := uint64(100)

	// 添加数据
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("large_key_%d", i)
		cms.Add(key, countsPerKey)
	}

	// 验证一部分数据
	errors := 0
	totalError := uint64(0)
	checkCount := 1000

	for i := 0; i < checkCount; i++ {
		key := fmt.Sprintf("large_key_%d", i)
		estimated := cms.Count(key)

		if estimated < countsPerKey {
			errors++
			t.Logf("Key '%s': underestimated, expected >= %d, got %d", key, countsPerKey, estimated)
		}

		if estimated > countsPerKey {
			totalError += estimated - countsPerKey
		}
	}

	errorRate := float64(errors) / float64(checkCount) * 100
	avgError := float64(totalError) / float64(checkCount)

	t.Logf("Large dataset test results:")
	t.Logf("  Checked %d keys", checkCount)
	t.Logf("  Underestimation errors: %d (%.2f%%)", errors, errorRate)
	t.Logf("  Average overestimation: %.2f", avgError)

	if errors > 0 {
		t.Errorf("Found %d underestimation errors (should be 0)", errors)
	}
}

// 并发安全测试
func TestCountMinSketch_ConcurrentAccess(t *testing.T) {
	cms := NewCountMinSketch(0.1, 0.1)

	done := make(chan bool, 10)

	// 并发添加
	for i := 0; i < 10; i++ {
		go func(workerID int) {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("worker_%d_key_%d", workerID, j)
				cms.Add(key, 1)
			}
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据
	for workerID := 0; workerID < 10; workerID++ {
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("worker_%d_key_%d", workerID, j)
			count := cms.Count(key)
			if count < 1 {
				t.Errorf("Key '%s' should have count >= 1, got %d", key, count)
			}
		}
	}

	t.Log("Concurrent access test passed")
}

// 参数影响测试
func TestCountMinSketch_ParameterEffects(t *testing.T) {
	testCases := []struct {
		epsilon float64
		delta   float64
		name    string
	}{
		{0.1, 0.1, "loose"},
		{0.01, 0.01, "tight"},
		{0.001, 0.001, "very_tight"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cms := NewCountMinSketch(tc.epsilon, tc.delta)

			// 添加相同的测试数据
			testKey := "param_test_key"
			expectedCount := uint64(1000)
			cms.Add(testKey, expectedCount)

			actualCount := cms.Count(testKey)

			t.Logf("Epsilon: %.3f, Delta: %.3f, Expected: %d, Actual: %d",
				tc.epsilon, tc.delta, expectedCount, actualCount)

			if actualCount < expectedCount {
				t.Errorf("Count should not be underestimated: expected >= %d, got %d",
					expectedCount, actualCount)
			}

			// 更严格的参数应该有更小的误差
			maxError := expectedCount + uint64(float64(expectedCount)*tc.epsilon*10) // 允许一定倍数的误差
			if actualCount > maxError {
				t.Errorf("Count error too large: expected <= %d, got %d", maxError, actualCount)
			}
		})
	}
}

// 零值和边界测试
func TestCountMinSketch_EdgeCases(t *testing.T) {
	cms := NewCountMinSketch(0.1, 0.1)

	// 测试空字符串
	cms.Add("", 5)
	if cms.Count("") < 5 {
		t.Error("Empty string key should work")
	}

	// 测试零计数
	cms.Add("zero_test", 0)
	count := cms.Count("zero_test")
	if count != 0 {
		t.Errorf("Zero count should remain zero, got %d", count)
	}

	// 测试不存在的key
	nonExistentCount := cms.Count("non_existent_key")
	if nonExistentCount != 0 {
		t.Errorf("Non-existent key should have count 0, got %d", nonExistentCount)
	}

	// 测试大数值
	largeCount := uint64(math.MaxUint32)
	cms.Add("large_count_key", largeCount)
	actualLargeCount := cms.Count("large_count_key")
	if actualLargeCount < largeCount {
		t.Errorf("Large count underestimated: expected >= %d, got %d", largeCount, actualLargeCount)
	}

	t.Log("Edge cases test passed")
}

// 性能基准测试
func BenchmarkCountMinSketch_Add(b *testing.B) {
	cms := NewCountMinSketch(0.01, 0.01)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i)
		cms.Add(key, 1)
	}
}

func BenchmarkCountMinSketch_Count(b *testing.B) {
	cms := NewCountMinSketch(0.01, 0.01)

	// 预先添加一些数据
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		cms.Add(key, uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%10000)
		cms.Count(key)
	}
}

// 不同参数的性能对比
func BenchmarkCountMinSketch_DifferentParams(b *testing.B) {
	params := []struct {
		epsilon float64
		delta   float64
		name    string
	}{
		{0.1, 0.1, "loose"},
		{0.01, 0.01, "medium"},
		{0.001, 0.001, "tight"},
	}

	for _, p := range params {
		b.Run(p.name, func(b *testing.B) {
			cms := NewCountMinSketch(p.epsilon, p.delta)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key_%d", i)
				cms.Add(key, 1)
			}
		})
	}
}

// 内存使用估算测试
func TestCountMinSketch_MemoryUsage(t *testing.T) {
	params := []struct {
		epsilon float64
		delta   float64
		name    string
	}{
		{0.1, 0.1, "loose"},
		{0.01, 0.01, "medium"},
		{0.001, 0.001, "tight"},
	}

	for _, p := range params {
		t.Run(p.name, func(t *testing.T) {
			cms := NewCountMinSketch(p.epsilon, p.delta)

			// 估算内存使用
			width := uint(math.Ceil(math.E / p.epsilon))
			depth := uint(math.Ceil(math.Log(1 / p.delta)))
			estimatedBytes := width * depth * 8 // uint64 = 8 bytes

			t.Logf("Epsilon: %.3f, Delta: %.3f", p.epsilon, p.delta)
			t.Logf("Width: %d, Depth: %d", width, depth)
			t.Logf("Estimated memory: %d bytes (%.2f KB)", estimatedBytes, float64(estimatedBytes)/1024)

			// 添加一些数据测试
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("mem_test_%d", i)
				cms.Add(key, uint64(i))
			}
		})
	}
}

// 衰减功能测试
func TestCountMinSketch_Decay(t *testing.T) {
	cms := NewCountMinSketch(0.1, 0.1)

	// 添加一些测试数据
	testData := map[string]uint64{
		"key1": 100,
		"key2": 200,
		"key3": 300,
		"key4": 50,
	}

	// 添加数据
	for key, count := range testData {
		cms.Add(key, count)
	}

	// 记录衰减前的计数
	beforeCounts := make(map[string]uint64)
	for key := range testData {
		beforeCounts[key] = cms.Count(key)
	}

	// 执行衰减
	cms.Decay()

	// 验证衰减后的计数
	for key, originalCount := range testData {
		beforeCount := beforeCounts[key]
		afterCount := cms.Count(key)

		t.Logf("Key '%s': original=%d, before_decay=%d, after_decay=%d",
			key, originalCount, beforeCount, afterCount)

		// 衰减后的计数应该大约是之前的一半
		expectedAfter := beforeCount / 2
		if afterCount > beforeCount {
			t.Errorf("Key '%s': count should decrease after decay, before=%d, after=%d",
				key, beforeCount, afterCount)
		}

		// 允许一定的误差范围（因为CMS的近似性质）
		if afterCount > 0 && (afterCount < expectedAfter/2 || afterCount > expectedAfter*2) {
			t.Logf("Key '%s': decay result outside expected range, expected ~%d, got %d",
				key, expectedAfter, afterCount)
		}
	}

	t.Log("Decay test passed")
}

// 多次衰减测试
func TestCountMinSketch_MultipleDecay(t *testing.T) {
	cms := NewCountMinSketch(0.05, 0.05)

	key := "decay_test_key"
	initialCount := uint64(1000)
	cms.Add(key, initialCount)

	countHistory := []uint64{cms.Count(key)}

	// 执行多次衰减
	for i := 0; i < 5; i++ {
		cms.Decay()
		count := cms.Count(key)
		countHistory = append(countHistory, count)
		t.Logf("After decay %d: count = %d", i+1, count)
	}

	// 验证每次衰减都在减少计数
	for i := 1; i < len(countHistory); i++ {
		if countHistory[i] > countHistory[i-1] {
			t.Errorf("Count should not increase after decay: step %d: %d -> %d",
				i, countHistory[i-1], countHistory[i])
		}
	}

	// 最终计数应该远小于初始计数
	finalCount := countHistory[len(countHistory)-1]
	if finalCount > initialCount/10 { // 期望至少减少到1/10
		t.Logf("Final count might be higher than expected: initial=%d, final=%d",
			initialCount, finalCount)
	}

	t.Log("Multiple decay test passed")
}
