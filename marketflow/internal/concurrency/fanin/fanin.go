package fanin

import (
	"marketflow/internal/domain/model"
	"sync"
)

// FanIn объединяет несколько каналов PriceUpdate в один.
// Когда все входные каналы закрыты, выходной канал закрывается.
func FanIn(channels ...<-chan model.PriceUpdate) <-chan model.PriceUpdate {
	out := make(chan model.PriceUpdate)
	var wg sync.WaitGroup
	wg.Add(len(channels))

	for _, ch := range channels {
		go func(c <-chan model.PriceUpdate) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
