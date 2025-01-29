package main

import (
	"fmt"
	"runtime"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

func main() {
	defer func() {
		time.Sleep(1 * time.Second)
		runtime.GC()

		var mem = runtime.MemStats{}
		runtime.ReadMemStats(&mem)
		fmt.Println("GC total STW ns:", mem.PauseTotalNs)
		fmt.Println("GC num GC:", mem.NumGC)
		fmt.Println("GC frees:", mem.Frees)
		fmt.Println("GC mallocs:", mem.Mallocs)
		fmt.Println("GC CPU fraction:", mem.GCCPUFraction)
		fmt.Println("GC fraction:")
	}()

	var val int
	for i := 0; i < 10000; i++ {
		go func() {
			for j := 0; j <= 5000; j++ {
				val++
			}
		}()
	}
}
