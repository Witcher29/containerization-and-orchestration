package main

import (
	"runtime"
	"time"
)

func main() {
	// Постепенно выделяем память и постоянно обращаемся к ней,
	// чтобы память действительно оставалась занятой.
	var memory [][]byte

	for {
		block := make([]byte, 10*1024*1024) // 10 MiB

		// Заполняем блок, чтобы страницы памяти реально использовались.
		for i := range block {
			block[i] = byte(i)
		}

		memory = append(memory, block)

		// Не даём GC освободить накопленную память.
		runtime.KeepAlive(memory)

		// Дополнительно грузим CPU.
		start := time.Now()
		for time.Since(start) < 100*time.Millisecond {
			for i := 0; i < 1_000_000; i++ {
				_ = i * i
			}
		}
	}
}