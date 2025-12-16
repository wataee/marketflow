package fanout

import "marketflow/internal/domain/model"

// FanOut распределяет данные из входного канала `in` по n выходным каналам.
// Возвращает срез выходных каналов. Каждый канал будет закрыт автоматически,
// когда входной канал закроется и все данные будут распределены.
func FanOut(in <-chan model.PriceUpdate, n int) []chan model.PriceUpdate {
	if n <= 0 {
		n = 1
	}
	outs := make([]chan model.PriceUpdate, n)
	for i := 0; i < n; i++ {
		outs[i] = make(chan model.PriceUpdate)
	}

	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()

		i := 0
		for v := range in {
			outs[i%n] <- v
			i++
		}
	}()

	return outs
}
