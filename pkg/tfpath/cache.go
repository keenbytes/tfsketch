package tfpath

type Cache struct {
	path string
}

func NewCache(path string) *Cache {
	cache := &Cache{
		path: path,
	}

	return cache
}
