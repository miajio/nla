package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {

	startUpdate := make(chan struct{})
	inventoryChain := make(chan time.Duration)
	inventoryTime := time.Second * 5

	ticker := time.NewTicker(inventoryTime)
	defer ticker.Stop()

	wg := sync.WaitGroup{}
	wg.Add(1)

	i := 10

	updateInventoryTime := func(newInventoryTime time.Duration) {
		inventoryChain <- newInventoryTime
	}

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				fmt.Println("tick", i)
				i--
				if i == 5 {
					startUpdate <- struct{}{}
				}
				if 0 == i {
					return
				}
			case newInventoryTime := <-inventoryChain:
				inventoryTime = newInventoryTime
				ticker.Stop()
				ticker = time.NewTicker(inventoryTime)
			}
		}
	}()

	go func() {
		select {
		case <-startUpdate:
			fmt.Println("start update")
			updateInventoryTime(time.Second * 2)
		}
	}()

	wg.Wait()
}
