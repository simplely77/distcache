package bloomfilter

import (
	"fmt"
	"testing"
)

// 基础功能测试
func TestBloomFilter_BasicOperations(t *testing.T) {
	bf := NewBloomFilter(1000, 3)

	// 测试添加和查询
	testKeys := []string{"key1", "key2", "key3", "hello", "world"}

	// 添加测试键
	for _, key := range testKeys {
		bf.Add(key)
	}

	// 验证添加的键都能找到
	for _, key := range testKeys {
		if !bf.Test(key) {
			t.Errorf("Expected key '%s' to be found in bloom filter", key)
		}
	}

	t.Logf("All %d keys found successfully", len(testKeys))
}

// 假阳性率测试（更合理的期望）
func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	// 使用更大的布隆过滤器减少假阳性率
	testCases := []struct {
		size      uint
		hashes    uint
		addCount  int
		testCount int
		name      string
		maxFPRate float64 // 最大可接受的假阳性率
	}{
		{10000, 3, 1000, 5000, "10K-3hash-1Kkeys", 50.0},
		{20000, 5, 1000, 5000, "20K-5hash-1Kkeys", 30.0},
		{50000, 7, 1000, 5000, "50K-7hash-1Kkeys", 15.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bf := NewBloomFilter(tc.size, tc.hashes)

			// 添加指定数量的键
			for i := 0; i < tc.addCount; i++ {
				key := fmt.Sprintf("added_key_%d", i)
				bf.Add(key)
			}

			// 测试未添加的键
			falsePositives := 0
			for i := 0; i < tc.testCount; i++ {
				key := fmt.Sprintf("test_key_%d", i+tc.addCount+10000) // 确保不冲突
				if bf.Test(key) {
					falsePositives++
				}
			}

			fpRate := float64(falsePositives) / float64(tc.testCount) * 100
			t.Logf("Size: %d, Hashes: %d, Added: %d, Tested: %d, FP Rate: %.2f%%",
				tc.size, tc.hashes, tc.addCount, tc.testCount, fpRate)

			if fpRate > tc.maxFPRate {
				t.Errorf("False positive rate too high: %.2f%% (max: %.2f%%)", fpRate, tc.maxFPRate)
			}
		})
	}
}

// 并发安全测试
func TestBloomFilter_ConcurrentAccess(t *testing.T) {
	bf := NewBloomFilter(50000, 5) // 使用更大的过滤器

	done := make(chan bool, 10)

	// 并发添加
	for i := 0; i < 10; i++ {
		go func(workerID int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("worker_%d_key_%d", workerID, j)
				bf.Add(key)
			}
		}(i)
	}

	// 等待所有添加完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	missing := 0
	for workerID := 0; workerID < 10; workerID++ {
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("worker_%d_key_%d", workerID, j)
			if !bf.Test(key) {
				missing++
				t.Errorf("Key '%s' should be found after concurrent add", key)
			}
		}
	}

	if missing == 0 {
		t.Logf("All 1000 concurrently added keys found successfully")
	}
}

// 边界条件测试
func TestBloomFilter_EdgeCases(t *testing.T) {
	bf := NewBloomFilter(1000, 3)

	// 测试空字符串
	bf.Add("")
	if !bf.Test("") {
		t.Error("Empty string should be found")
	}

	// 测试重复添加
	key := "duplicate_test"
	for i := 0; i < 10; i++ {
		bf.Add(key)
	}
	if !bf.Test(key) {
		t.Error("Key should be found after multiple additions")
	}

	// 测试特殊字符
	specialKeys := []string{
		"测试中文",
		"key\nwith\nnewlines",
		"key\twith\ttabs",
		"key with spaces",
	}

	for _, key := range specialKeys {
		bf.Add(key)
		if !bf.Test(key) {
			t.Errorf("Special key '%s' should be found", key)
		}
	}

	t.Log("All edge cases passed")
}

// 容量测试
func TestBloomFilter_Capacity(t *testing.T) {
	// 测试不同大小的布隆过滤器
	sizes := []struct {
		size   uint
		hashes uint
		keys   int
		name   string
	}{
		{1000, 3, 100, "small"},
		{10000, 5, 1000, "medium"},
		{100000, 7, 10000, "large"},
	}

	for _, s := range sizes {
		t.Run(s.name, func(t *testing.T) {
			bf := NewBloomFilter(s.size, s.hashes)

			// 添加指定数量的键
			for i := 0; i < s.keys; i++ {
				key := fmt.Sprintf("%s_key_%d", s.name, i)
				bf.Add(key)
			}

			// 验证所有键都能找到
			notFound := 0
			for i := 0; i < s.keys; i++ {
				key := fmt.Sprintf("%s_key_%d", s.name, i)
				if !bf.Test(key) {
					notFound++
				}
			}

			if notFound > 0 {
				t.Errorf("Failed to find %d out of %d keys", notFound, s.keys)
			} else {
				t.Logf("Successfully found all %d keys in %s filter", s.keys, s.name)
			}
		})
	}
}

// 性能基准测试
func BenchmarkBloomFilter_Add(b *testing.B) {
	bf := NewBloomFilter(100000, 5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i)
		bf.Add(key)
	}
}

func BenchmarkBloomFilter_Test(b *testing.B) {
	bf := NewBloomFilter(100000, 5)

	// 预先添加一些数据
	for i := 0; i < 10000; i++ {
		bf.Add(fmt.Sprintf("key_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("test_key_%d", i)
		bf.Test(key)
	}
}

// 不同哈希函数数量的性能对比
func BenchmarkBloomFilter_DifferentHashCount(b *testing.B) {
	hashCounts := []uint{1, 3, 5, 7}

	for _, hashCount := range hashCounts {
		b.Run(fmt.Sprintf("Hash_%d", hashCount), func(b *testing.B) {
			bf := NewBloomFilter(50000, hashCount)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key_%d", i)
				bf.Add(key)
			}
		})
	}
}

// 真实场景模拟测试
func TestBloomFilter_RealWorldScenario(t *testing.T) {
	// 模拟缓存场景：100万个可能的key，10万个实际存在的key
	bf := NewBloomFilter(1000000, 7) // 1M位，7个哈希函数

	// 添加10万个"存在"的key
	existingKeys := make(map[string]bool)
	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("existing_key_%d", i)
		bf.Add(key)
		existingKeys[key] = true
	}

	// 测试1万个随机key的查询
	falsePositives := 0
	truePositives := 0
	trueNegatives := 0

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("random_key_%d", i+200000) // 避免与existing keys冲突

		result := bf.Test(key)
		if existingKeys[key] {
			if result {
				truePositives++
			}
		} else {
			if result {
				falsePositives++
			} else {
				trueNegatives++
			}
		}
	}

	fpRate := float64(falsePositives) / float64(falsePositives+trueNegatives) * 100

	t.Logf("Real world scenario results:")
	t.Logf("  True Positives: %d", truePositives)
	t.Logf("  True Negatives: %d", trueNegatives)
	t.Logf("  False Positives: %d", falsePositives)
	t.Logf("  False Positive Rate: %.2f%%", fpRate)

	// 在真实场景中，假阳性率应该比较低
	if fpRate > 10.0 {
		t.Errorf("False positive rate too high for real world scenario: %.2f%%", fpRate)
	}
}
