package storage_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"go-lancache/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cache = storage.NewStorageCache(time.Minute)

func TestGet_NotFound(t *testing.T) {
	_, ok := cache.Get("unknown")
	assert.False(t, ok)
}

func Test_SetGet(t *testing.T) {
	s1 := storage.Storage{
		BasePath: "second",
	}

	cache.Set(s1.BasePath, s1)

	s2, ok := cache.Get(s1.BasePath)
	assert.True(t, ok)
	assert.Equal(t, s1.BasePath, s2.BasePath)
}

func TestUpdate_Change(t *testing.T) {
	s := storage.Storage{
		BasePath:      "changed",
		DownloadCount: 1,
	}
	cache.Set(s.BasePath, s)

	updated := cache.Update(s.BasePath, func(s *storage.Storage) *storage.Storage {
		s.DownloadCount = 10
		return s
	})
	assert.NotNil(t, updated)
	assert.Equal(t, updated.BasePath, "changed")
	assert.Equal(t, updated.DownloadCount, 10)
	assert.Equal(t, s.DownloadCount, 1)

	got, ok := cache.Get(s.BasePath)
	assert.True(t, ok)
	assert.Equal(t, got.BasePath, "changed")
	assert.Equal(t, got.DownloadCount, 10)
}

func TestUpdate_NotChange(t *testing.T) {
	s := storage.Storage{
		BasePath:      "notChanged",
		DownloadCount: 1,
	}
	cache.Set(s.BasePath, s)

	updated := cache.Update(s.BasePath, func(s *storage.Storage) *storage.Storage {
		s.DownloadCount = 10
		return nil
	})
	assert.NotNil(t, updated)
	assert.Equal(t, updated.BasePath, "notChanged")
	assert.Equal(t, updated.DownloadCount, 10)

	got, ok := cache.Get(s.BasePath)
	assert.True(t, ok)
	assert.Equal(t, got.BasePath, "notChanged")
	assert.Equal(t, got.DownloadCount, 1)
}

func TestUpdate_NotFound(t *testing.T) {
	updated := cache.Update("missing", func(s *storage.Storage) *storage.Storage {
		return s
	})

	require.Nil(t, updated)
}

func Test_FirstLast_one_item(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	s := storage.Storage{
		BasePath: "first",
	}
	cache.Set(s.BasePath, s)

	first := cache.GetFirst()
	last := cache.GetLast()
	require.NotNil(t, first)
	require.NotNil(t, last)
	assert.Equal(t, first.Key, s.BasePath)
	assert.Equal(t, last.Key, s.BasePath)
	assert.Same(t, first, last)

	cache.Get(s.BasePath)
	require.Nil(t, first.Next)
	require.Nil(t, first.Prev)
}

func Test_FirstLast_two_items(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	s1 := storage.Storage{
		BasePath: "first",
	}
	cache.Set(s1.BasePath, s1)
	s2 := storage.Storage{
		BasePath: "second",
	}
	cache.Set(s2.BasePath, s2)

	first := cache.GetFirst()
	last := cache.GetLast()
	require.Equal(t, first.Key, s1.BasePath)
	require.Equal(t, last.Key, s2.BasePath)

	require.Nil(t, first.Prev)
	require.Nil(t, last.Next)
	require.Equal(t, first.Next.Key, s2.BasePath)
	require.Equal(t, last.Prev.Key, s1.BasePath)

	cache.Get(s1.BasePath)
	first = cache.GetFirst()
	last = cache.GetLast()
	require.Equal(t, first.Key, s2.BasePath)
	require.Equal(t, last.Key, s1.BasePath)

	require.Nil(t, first.Prev)
	require.Nil(t, last.Next)
	require.Equal(t, first.Next.Key, s1.BasePath)
	require.Equal(t, last.Prev.Key, s2.BasePath)
}

func Test_FirstLast_three_items(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	s1 := storage.Storage{
		BasePath: "first",
	}
	cache.Set(s1.BasePath, s1)
	s2 := storage.Storage{
		BasePath: "second",
	}
	cache.Set(s2.BasePath, s2)
	s3 := storage.Storage{
		BasePath: "third",
	}
	cache.Set(s3.BasePath, s3)

	first := cache.GetFirst()
	last := cache.GetLast()
	require.Equal(t, first.Key, s1.BasePath)
	require.Equal(t, last.Key, s3.BasePath)

	require.Nil(t, first.Prev)
	require.Nil(t, last.Next)
	require.Equal(t, first.Next.Key, s2.BasePath)
	require.Equal(t, last.Prev.Key, s2.BasePath)
	require.Equal(t, first.Next.Prev.Key, s1.BasePath)
	require.Equal(t, first.Next.Next.Key, s3.BasePath)

	cache.Get(s2.BasePath)
	first = cache.GetFirst()
	last = cache.GetLast()
	require.Equal(t, first.Key, s1.BasePath)
	require.Equal(t, last.Key, s2.BasePath)

	require.Nil(t, first.Prev)
	require.Nil(t, last.Next)
	require.Equal(t, first.Next.Key, s3.BasePath)
	require.Equal(t, last.Prev.Key, s3.BasePath)
	require.Equal(t, first.Next.Prev.Key, s1.BasePath)
	require.Equal(t, first.Next.Next.Key, s2.BasePath)
}

func TestFirstLast_Orders(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	for i := range 20 {
		s := storage.Storage{
			BasePath: "key" + strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	next := cache.GetFirst()
	for i := range 20 {
		require.NotNil(t, next)
		assert.Equal(t, next.Key, "key"+strconv.Itoa(i))
		next = next.Next
	}

	last := cache.GetLast()
	assert.Equal(t, last.Key, "key19")
}

func TestFirstLast_MoveItems(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	for i := range 20 {
		s := storage.Storage{
			BasePath: "key" + strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	cache.Get("key15")
	last := cache.GetLast()
	require.Equal(t, "key15", last.Key)
	require.Equal(t, "key19", last.Prev.Key)

	cache.Get("key5")
	last = cache.GetLast()
	require.Equal(t, "key5", last.Key)
	require.Equal(t, "key15", last.Prev.Key)

	cache.Get("key10")
	last = cache.GetLast()
	require.Equal(t, "key10", last.Key)
	require.Equal(t, "key5", last.Prev.Key)
	assert.Equal(t, "key15", last.Prev.Prev.Key)

	next := cache.GetFirst()
	for range 17 {
		require.NotNil(t, next)
		assert.NotEqual(t, "key5", next.Key)
		assert.NotEqual(t, "key10", next.Key)
		assert.NotEqual(t, "key15", next.Key)

		if next.Key == "key4" {
			assert.Equal(t, "key6", next.Next.Key)
			assert.Equal(t, "key4", next.Next.Prev.Key)
		}
		if next.Key == "key9" {
			assert.Equal(t, "key11", next.Next.Key)
			assert.Equal(t, "key9", next.Next.Prev.Key)
		}
		if next.Key == "key14" {
			assert.Equal(t, "key16", next.Next.Key)
			assert.Equal(t, "key14", next.Next.Prev.Key)
		}
		next = next.Next
	}
}

func TestFirstLast_MoveFirst(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	for i := range 20 {
		s := storage.Storage{
			BasePath: "key" + strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	cache.Get("key0")

	first := cache.GetFirst()
	require.Equal(t, "key1", first.Key)
	require.Equal(t, "key2", first.Next.Key)
	require.Nil(t, first.Prev)

	last := cache.GetLast()
	require.Equal(t, "key0", last.Key)
	require.Equal(t, "key19", last.Prev.Key)
	require.Nil(t, last.Next)
}

func TestFirstLast_MoveLast(t *testing.T) {
	cache := storage.NewStorageCache(time.Minute)

	for i := range 20 {
		s := storage.Storage{
			BasePath: "key" + strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	cache.Get("key19")

	first := cache.GetFirst()
	require.Equal(t, "key0", first.Key)
	require.Equal(t, "key1", first.Next.Key)
	require.Nil(t, first.Prev)

	last := cache.GetLast()
	require.Equal(t, "key19", last.Key)
	require.Equal(t, "key18", last.Prev.Key)
	require.Nil(t, last.Next)
}

func Test_Expiration(t *testing.T) {
	cache := storage.NewStorageCache(time.Second)
	cache.Start(context.Background())
	defer cache.Stop()

	cache.Set("key", storage.Storage{
		BasePath: "test",
	})

	time.Sleep(3 * time.Second)

	_, ok := cache.Get("key")
	require.False(t, ok)
}

func TestUpdate_UpTime(t *testing.T) {
	cache := storage.NewStorageCache(time.Second)
	cache.Start(context.Background())
	defer cache.Stop()

	cache.Set("key", storage.Storage{
		BasePath: "test",
	})

	time.Sleep(700 * time.Millisecond)

	cache.Update("key", func(s *storage.Storage) *storage.Storage {
		s.BasePath = "updated"
		return s
	})

	time.Sleep(500 * time.Millisecond)

	got, ok := cache.Get("key")
	require.True(t, ok)
	require.Equal(t, "updated", got.BasePath)
}

func TestGet_UpTime(t *testing.T) {
	cache := storage.NewStorageCache(300 * time.Millisecond)
	cache.Start(context.Background())
	defer cache.Stop()

	cache.Set("key", storage.Storage{
		BasePath: "test",
	})

	time.Sleep(290 * time.Millisecond)

	_, ok := cache.Get("key")
	require.True(t, ok)

	time.Sleep(500 * time.Millisecond)

	_, ok = cache.Get("key")
	require.True(t, ok)
}

func TestSet_UpTime(t *testing.T) {
	cache := storage.NewStorageCache(time.Second)
	cache.Start(context.Background())
	defer cache.Stop()

	cache.Set("key", storage.Storage{
		BasePath: "test",
	})

	time.Sleep(700 * time.Millisecond)

	cache.Set("key", storage.Storage{
		BasePath: "new",
	})

	time.Sleep(500 * time.Millisecond)

	got, ok := cache.Get("key")
	require.True(t, ok)
	require.Equal(t, "new", got.BasePath)
}

func waitUntil(t *testing.T, fn func() bool) {
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition not met")
}

func TestUncache_All(t *testing.T) {
	cache := storage.NewStorageCache(time.Second)
	cache.Start(context.Background())

	for i := range 100 {
		s := storage.Storage{
			BasePath: strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	time.Sleep(500 * time.Millisecond)

	for range 50 {
		key := strconv.Itoa(rand.Intn(100))

		_, ok := cache.Get(key)
		require.True(t, ok)
	}

	time.Sleep(1200 * time.Millisecond)
	cache.Stop()

	require.Nil(t, cache.GetFirst())
	require.Nil(t, cache.GetLast())
}

func TestUncache_OrderValidate(t *testing.T) {
	cache := storage.NewStorageCache(time.Second)
	cache.Start(context.Background())

	for i := range 100 {
		s := storage.Storage{
			BasePath: strconv.Itoa(i),
		}
		cache.Set(s.BasePath, s)
	}

	time.Sleep(500 * time.Millisecond)

	keys := make([]string, 0, 100)
	visited := make(map[string]struct{})

	for range 50 {
		key := strconv.Itoa(rand.Intn(50))

		if _, ok := visited[key]; ok {
			continue
		}

		_, ok := cache.Get(key)
		require.True(t, ok)
		keys = append(keys, key)
		visited[key] = struct{}{}
	}

	time.Sleep(500 * time.Millisecond)

	for range 50 {
		key := strconv.Itoa(rand.Intn(50) + 50)

		if _, ok := visited[key]; ok {
			continue
		}

		_, ok := cache.Get(key)
		require.True(t, ok)
		keys = append(keys, key)
		visited[key] = struct{}{}
	}

	waitUntil(t, func() bool {
		return cache.Size() == len(keys)
	})

	cache.Stop()

	require.Equal(t, len(keys), cache.Size())
	require.Equal(t, keys[len(keys)-1], cache.GetLast().Key)
	require.NoError(t, cache.ValidateList())

	cur := cache.GetFirst()
	i := 0
	for cur != nil {
		require.Equal(t, keys[i], cur.Key)
		i++
		cur = cur.Next
	}
}

func TestUncache_OnEvict(t *testing.T) {
	// var wg sync.WaitGroup
	chEvictKeys := make(chan string)
	visited := make(map[string]struct{}, 50)

	cache := storage.NewStorageCache(time.Second)
	cache.OnEvict = func(s storage.Storage) {
		// defer wg.Done()
		// _, ok := visited[s.BasePath]
		// require.False(t, ok)
		chEvictKeys <- s.BasePath
	}
	cache.Start(context.Background())
	defer cache.Stop()

	for i := range 100 {
		s := storage.Storage{
			BasePath: strconv.Itoa(i),
			Dirty:    true,
		}
		cache.Set(s.BasePath, s)
	}

	time.Sleep(500 * time.Millisecond)

	for range 50 {
		key := strconv.Itoa(rand.Intn(100))
		cache.Get(key)
		visited[key] = struct{}{}
	}

	count := 100 - len(visited)
	for key := range chEvictKeys {
		_, ok := visited[key]
		require.False(t, ok)
		count--
		if count == 0 {
			close(chEvictKeys)
		}
	}

	// wg.Add(100 - len(visited))
	// wg.Wait()

}

func Test_ConcurrentAccess(t *testing.T) {
	cache.Set("main", storage.Storage{})

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 10000 {
				switch j % 3 {
				case 0:
					cache.Get("main")

				case 1:
					cache.Set("main", storage.Storage{})

				case 2:
					cache.Update("main", func(s *storage.Storage) *storage.Storage {
						copy := *s
						return &copy
					})
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestSetGet_ManyKeys(t *testing.T) {
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("key-%d", id)

			for range 5000 {
				cache.Set(key, storage.Storage{})
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func Test_ListIntegrity(t *testing.T) {
	for i := range 1000 {
		key := fmt.Sprintf("%d", i)
		cache.Set(key, storage.Storage{})
	}

	for range 10000 {
		key := fmt.Sprintf("%d", rand.Intn(1000))
		cache.Get(key)
	}

	require.NoError(t, cache.ValidateList())
}

func Test_ListIntegrityConcurrent(t *testing.T) {
	for i := range 1000 {
		cache.Set(strconv.Itoa(i), storage.Storage{})
	}

	var wg sync.WaitGroup

	for range 32 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 10000 {
				key := strconv.Itoa(rand.Intn(1000))

				cache.Get(key)

				if j%100 == 0 {
					cache.Set(key, storage.Storage{})
				}
			}
		}()
	}

	wg.Wait()

	require.NoError(t, cache.ValidateList())
}

func Test_CountMatchesList(t *testing.T) {
	cache := storage.NewStorageCache(time.Hour)
	cache.Start(context.Background())
	defer cache.Stop()

	for i := range 1000 {
		cache.Set(strconv.Itoa(i), storage.Storage{})
	}

	for range 50000 {
		cache.Get(strconv.Itoa(rand.Intn(1000)))
	}

	mapCount := cache.Size()

	listCount := 0
	cur := cache.GetFirst()

	for cur != nil {
		listCount++
		cur = cur.Next
	}

	require.Equal(t, mapCount, listCount)
}

func Test_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	cache := storage.NewStorageCache(5 * time.Second)
	cache.Start(context.Background())
	defer cache.Stop()

	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100000 {
				key := strconv.Itoa(rand.Intn(10000))

				switch rand.Intn(3) {
				case 0:
					cache.Get(key)

				case 1:
					cache.Set(key, storage.Storage{})

				case 2:
					cache.Update(key, func(s *storage.Storage) *storage.Storage {
						return s
					})
				}
			}
		}()
	}

	wg.Wait()

	require.NoError(t, cache.ValidateList())
}

func init() {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	slog.SetDefault(log)
	cache.Start(context.Background())
}
