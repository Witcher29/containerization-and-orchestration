package main

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

// envInt читает целочисленную переменную окружения или возвращает дефолт.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envBool читает булеву переменную окружения или возвращает дефолт.
func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func main() {
	blockMiB := envInt("BATCH_BLOCK_MIB", 10)
	targetMiB := envInt("BATCH_MEMORY_TARGET_MIB", 200)
	cpuBurn := envBool("BATCH_CPU_BURN", true)
	allocInterval := time.Duration(envInt("BATCH_ALLOC_INTERVAL_MS", 100)) * time.Millisecond

	if blockMiB <= 0 {
		blockMiB = 10
	}
	if targetMiB < blockMiB {
		targetMiB = blockMiB
	}

	blockSize := blockMiB * 1024 * 1024
	maxBlocks := targetMiB / blockMiB

	log.Printf("batch starting: block=%dMiB target=%dMiB maxBlocks=%d cpuBurn=%v allocInterval=%s",
		blockMiB, targetMiB, maxBlocks, cpuBurn, allocInterval)

	// Плато памяти: аллоцируем до targetMiB и держим блоки живыми.
	memory := make([][]byte, 0, maxBlocks)

	for len(memory) < maxBlocks {
		block := make([]byte, blockSize)
		for i := range block {
			block[i] = byte(i)
		}
		memory = append(memory, block)
		runtime.KeepAlive(memory)

		// Даём kubelet/планировщику время увидеть рост и не убить под мгновенно.
		time.Sleep(allocInterval)
	}

	log.Printf("batch reached plateau: %d blocks, ~%d MiB resident", len(memory), len(memory)*blockMiB)

	// Плато: держим память, жжём CPU, периодически трогаем страницы.
	for {
		if cpuBurn {
			// ~500 мс насыщения одного ядра за итерацию.
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				for i := 0; i < 1_000_000; i++ {
					_ = i * i
				}
			}
		} else {
			time.Sleep(time.Second)
		}

		// Касаемся первого байта каждого блока, чтобы страницы оставались resident
		// и запись не была оптимизирована компилятором.
		now := byte(time.Now().UnixNano())
		for _, block := range memory {
			block[0] = now
		}
		runtime.KeepAlive(memory)
	}
}
