package lancache_test

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-lancache/internal/config"
	"go-lancache/internal/lancache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defIp = "192.168.1.1"
var defKey = "test"

func getDetector(ttl, max, timeTicker int) *lancache.AnomalyDetector {
	cfg := config.AnomalyDetectorConfig{
		HistoryTTL:        ttl,
		HistoryMax:        max,
		CleanupTimeTicker: timeTicker,
	}

	return lancache.NewAnomalyDetector(cfg, time.Duration(ttl)*time.Millisecond, time.Duration(timeTicker)*time.Millisecond)
}

func Test_SimpleDetect(t *testing.T) {
	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey)
	require.False(t, res)
}

func Test_CheckDetectIp(t *testing.T) {
	detector := getDetector(60000, 2, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey)
	require.False(t, res)

	res = detector.Detect(defIp, defKey)
	require.False(t, res)

	res = detector.Detect(defIp, defKey)
	require.True(t, res)

}

func TestCleanup(t *testing.T) {
	detector := getDetector(2000, 3, 5000)
	detector.Start(context.Background())
	defer detector.Stop()

	for i := range 100 {
		for range 10000 {
			key := strconv.Itoa(rand.Intn(10000))
			detector.Detect(strconv.Itoa(i), key)
			detector.Finish(strconv.Itoa(i), key)
		}
	}
}

func Test_Expiration(t *testing.T) {
	detector := getDetector(300, 3, 200)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey)
	require.False(t, res)

	detector.Finish(defIp, defKey)

	time.Sleep(300 * time.Millisecond)

	history, err := detector.Get(defIp, defKey)
	require.NoError(t, err)
	assert.Equal(t, 1, history.Times)

	time.Sleep(150 * time.Millisecond)

	history, err = detector.Get(defIp, defKey)
	require.Error(t, err)
}

func TestCleanup_NotRemoveLive(t *testing.T) {
	detector := getDetector(100, 3, 200)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey)
	require.False(t, res)

	time.Sleep(300 * time.Millisecond)

	history, err := detector.Get(defIp, defKey)
	require.NoError(t, err)
	assert.Equal(t, 1, history.Times)
}

func TestCleanup_CheckLastFinished(t *testing.T) {
	detector := getDetector(300, 3, 200)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey)
	require.False(t, res)
	detector.Finish(defIp, defKey)

	time.Sleep(250 * time.Millisecond)

	res = detector.Detect(defIp, defKey)
	require.False(t, res)
	detector.Finish(defIp, defKey)

	history, err := detector.Get(defIp, defKey)
	require.NoError(t, err)
	assert.Equal(t, 2, history.Times)

	time.Sleep(400 * time.Millisecond)

	history, err = detector.Get(defIp, defKey)
	require.Error(t, err)
}

func Test_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	var wg sync.WaitGroup

	for i := range 1000 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for range 10000 {
				key := strconv.Itoa(rand.Intn(10000))
				detector.Detect(strconv.Itoa(id), key)
				detector.Finish(strconv.Itoa(id), key)
			}
		}(i)
	}

	wg.Wait()
}

func BenchmarkTrack(b *testing.B) {
	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	for i := 0; i < b.N; i++ {
		ip := strconv.Itoa(i % 255)
		key := strconv.Itoa(i % 10000)

		detector.Detect(ip, key)
		detector.Finish(ip, key)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		wg.Done()
		detector.GetAccessList()
	}()

	wg.Wait()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ip := strconv.Itoa(i % 255)
		key := strconv.Itoa(i % 10000)

		detector.Detect(ip, key)
		detector.Finish(ip, key)
	}
}

func BenchmarkTrackParallel(b *testing.B) {
	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	var counter atomic.Uint64

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := counter.Add(1)

			ip := strconv.FormatUint(n%255, 10)
			key := strconv.FormatUint(n%10000, 10)

			detector.Detect(ip, key)
			detector.Finish(ip, key)
		}
	})
}

func BenchmarkTrackSameIP(b *testing.B) {
	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	var counter atomic.Uint64

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := strconv.FormatUint(counter.Add(1)%10000, 10)
			detector.Detect("192.168.0.1", key)
			detector.Finish("192.168.0.1", key)
		}
	})
}
