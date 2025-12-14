package fanout

import "marketflow/internal/domain/model"

// FanOut распределяет данные из одного канала в несколько
func FanOut(in <-chan model.PriceUpdate, n int) []chan model.PriceUpdate {
	outs := make([]chan model.PriceUpdate, n)
	for i := range outs {
		outs[i] = make(chan model.PriceUpdate)
	}

	go func() {
		defer func() {
			for _, out := range outs {
				close(out)
			}
		}()

		i := 0
		for price := range in {
			outs[i%n] <- price
			i++
		}
	}()

	return outs
}
