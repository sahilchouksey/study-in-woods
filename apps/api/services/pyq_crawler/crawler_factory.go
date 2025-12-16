package pyq_crawler

import (
	"fmt"
	"sync"
)

// CrawlerFactory manages crawler instances
type CrawlerFactory struct {
	crawlers map[string]PYQCrawlerInterface
	mu       sync.RWMutex
}

// Global factory instance
var (
	factory     *CrawlerFactory
	factoryOnce sync.Once
)

// GetCrawlerFactory returns the singleton factory instance
func GetCrawlerFactory() *CrawlerFactory {
	factoryOnce.Do(func() {
		factory = &CrawlerFactory{
			crawlers: make(map[string]PYQCrawlerInterface),
		}
		// Register default crawlers
		factory.RegisterCrawler(NewRGPVCrawler())
	})
	return factory
}

// RegisterCrawler registers a new crawler instance
func (f *CrawlerFactory) RegisterCrawler(crawler PYQCrawlerInterface) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.crawlers[crawler.GetName()] = crawler
}

// GetCrawler retrieves a crawler by name
func (f *CrawlerFactory) GetCrawler(name string) (PYQCrawlerInterface, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	crawler, exists := f.crawlers[name]
	if !exists {
		return nil, fmt.Errorf("crawler '%s' not found", name)
	}

	return crawler, nil
}

// GetAllCrawlers returns all registered crawlers
func (f *CrawlerFactory) GetAllCrawlers() []PYQCrawlerInterface {
	f.mu.RLock()
	defer f.mu.RUnlock()

	crawlers := make([]PYQCrawlerInterface, 0, len(f.crawlers))
	for _, crawler := range f.crawlers {
		crawlers = append(crawlers, crawler)
	}

	return crawlers
}

// GetCrawlerNames returns all registered crawler names
func (f *CrawlerFactory) GetCrawlerNames() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.crawlers))
	for name := range f.crawlers {
		names = append(names, name)
	}

	return names
}

// UnregisterCrawler removes a crawler from the factory
func (f *CrawlerFactory) UnregisterCrawler(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.crawlers[name]; !exists {
		return fmt.Errorf("crawler '%s' not found", name)
	}

	delete(f.crawlers, name)
	return nil
}
