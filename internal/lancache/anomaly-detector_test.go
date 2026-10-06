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
var withoutRange = ""

func getDetector(ttl, max, timeTicker int) *lancache.AnomalyDetector {
	cfg := config.AnomalyDetectorConfig{
		HistoryTTL:        ttl,
		HistoryMax:        max,
		CleanupTimeTicker: timeTicker,
	}

	return lancache.NewAnomalyDetector(cfg, time.Duration(ttl)*time.Millisecond, time.Duration(timeTicker)*time.Millisecond)
}

func Test_Detect_Simple(t *testing.T) {
	detector := getDetector(60000, 3, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey, withoutRange)
	require.False(t, res)
}

func Test_Detect_CheckIp(t *testing.T) {
	detector := getDetector(60000, 2, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey, withoutRange)
	require.False(t, res)
	res = detector.Detect(defIp, defKey, withoutRange)
	require.False(t, res)

	res = detector.Detect(defIp, defKey, withoutRange)
	require.True(t, res)
}

func Test_Detect_CheckIpAndRange(t *testing.T) {
	detector := getDetector(60000, 2, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	rangeHeader := "bytes=0-10000,30000-60000"

	res := detector.Detect(defIp, defKey, rangeHeader)
	require.False(t, res)
	res = detector.Detect(defIp, defKey, rangeHeader)
	require.False(t, res)

	res = detector.Detect(defIp, defKey, rangeHeader)
	require.True(t, res)
}

func Test_Detect_CheckIpAndAnyRange(t *testing.T) {
	detector := getDetector(60000, 2, 120000)
	detector.Start(context.Background())
	defer detector.Stop()

	rangeHeader1 := "bytes=0-10000,30000-60000"
	rangeHeader2 := "bytes=7999-87777"

	res := detector.Detect(defIp, defKey, rangeHeader1)
	require.False(t, res)
	res = detector.Detect(defIp, defKey, rangeHeader2)
	require.False(t, res)

	history, err := detector.Get(defIp, defKey)
	require.NoError(t, err)

	for _, times := range history.RangeTimes {
		assert.Equal(t, 1, times)
	}
}

func TestCleanup(t *testing.T) {
	detector := getDetector(2000, 3, 5000)
	detector.Start(context.Background())
	defer detector.Stop()

	for i := range 100 {
		for range 10000 {
			key := strconv.Itoa(rand.Intn(10000))
			detector.Detect(strconv.Itoa(i), key, withoutRange)
			detector.Finish(strconv.Itoa(i), key)
		}
	}
}

func Test_Expiration(t *testing.T) {
	detector := getDetector(300, 3, 200)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey, withoutRange)
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

func Test_Cleanup_NotRemoveLive(t *testing.T) {
	detector := getDetector(100, 3, 200)
	detector.Start(context.Background())
	defer detector.Stop()

	res := detector.Detect(defIp, defKey, withoutRange)
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

	res := detector.Detect(defIp, defKey, withoutRange)
	require.False(t, res)
	detector.Finish(defIp, defKey)

	time.Sleep(250 * time.Millisecond)

	res = detector.Detect(defIp, defKey, withoutRange)
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

	for i := range 100 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for range 10000 {
				key := strconv.Itoa(rand.Intn(10000))
				detector.Detect(strconv.Itoa(id), key, withoutRange)
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

		detector.Detect(ip, key, withoutRange)
		detector.Finish(ip, key)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ip := strconv.Itoa(i % 255)
		key := strconv.Itoa(i % 10000)

		detector.Detect(ip, key, withoutRange)
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

			detector.Detect(ip, key, withoutRange)
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
			detector.Detect(defIp, key, withoutRange)
			detector.Finish(defIp, key)
		}
	})
}
