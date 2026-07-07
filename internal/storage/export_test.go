package storage

import "fmt"

func (c *StorageCache) GetFirst() *cacheNode {
	return c.first
}

func (c *StorageCache) GetLast() *cacheNode {
	return c.last
}

func (c *StorageCache) ValidateList() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	visited := make(map[*cacheNode]struct{})

	cur := c.first
	var prev *cacheNode

	for cur != nil {
		_, ok := visited[cur]
		if ok {
			return fmt.Errorf("cycle detected")
		}

		visited[cur] = struct{}{}

		if cur.Prev != prev {
			curPrevKey := "nil"
			if cur.Prev != nil {
				curPrevKey = cur.Prev.Key
			}
			prevKey := "nil"
			if prev != nil {
				prevKey = prev.Key
			}
			return fmt.Errorf("broken prev link %s => %s", curPrevKey, prevKey)
		}

		prev = cur
		cur = cur.Next
	}

	if prev != c.last {
		return fmt.Errorf("last pointer mismatch")
	}

	if len(c.items) != len(visited) {
		return fmt.Errorf("items size and visited size mismatch")
	}

	return nil
}

func (c *StorageCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}
