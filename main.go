package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	n := int64(1)
	defer close(ch) // Закрываем канал перед завершением работы
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ch <- n
			fn(n)
			n++
		}
	}
}

func Worker(in <-chan int64, out chan<- int64) {
	for {
		n, ok := <-in
		if !ok {
			close(out)
			return
		}
		out <- n
		time.Sleep(1 * time.Millisecond)
	}
}

func main() {
	chIn := make(chan int64)

	ctx, cancel := context.WithTimeout(context.Background(), 1e9)
	defer cancel()

	var inputSum int64   // сумма сгенерированных чисел
	var inputCount int64 // количество сгенерированных чисел

	// Генератор чисел, исправлена гонка
	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)   // была гонка
		atomic.AddInt64(&inputCount, 1) // была гонка
	})

	const NumOut = 5
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	amounts := make([]int64, NumOut)
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	for i, out := range outs {
		wg.Add(1)
		go func(in <-chan int64, i int) {
			defer wg.Done()
			for n := range in {
				amounts[i]++
				chOut <- n
			}
		}(out, i)
	}

	go func() {
		wg.Wait()
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	for n := range chOut {
		count++
		sum += n
	}
	fmt.Println("Количество чисел", inputCount, count)
	fmt.Println("Сумма чисел", inputSum, sum)
	fmt.Println("Разбивка по каналам", amounts)

	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}
	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
