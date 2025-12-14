package fanin

import (
	"marketflow/internal/domain/model"
	"sync"
)

// FanIn объединяет несколько каналов в один
func FanIn(channels ...<-chan model.PriceUpdate) <-chan model.PriceUpdate {
	out := make(chan model.PriceUpdate)
	var wg sync.WaitGroup

	wg.Add(len(channels))

	for _, ch := range channels {
		go func(c <-chan model.PriceUpdate) {
			defer wg.Done()
			for price := range c {
				out <- price
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
