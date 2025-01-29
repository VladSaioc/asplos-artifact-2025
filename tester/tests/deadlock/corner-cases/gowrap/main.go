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
	ch := make(chan any)
	// main.main.gowrap1
	defer close(make(chan any))
	// main.main.gowrap2
	go close(make(chan any))
	// main.main.gowrap3
	go println()
	// main.main.gowrap4
	defer println()

	defer func() { // main.main.func1
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
	}()

	go func() { // main.main.func2
		// deadlocks: 1
		<-ch
	}()

	// main.main.gowrap5
	go func(_ int) { // main.main.func3
		// deadlocks: 1
		<-ch
	}(0)

	// main.main.gowrap6
	go (func(_ int) { // main.main.func4
		// deadlocks: 1
		<-ch
	})(0)

	// main.main.gowrap7
	go func(_ int) {
		go func() { // main.main.func5
			// deadlocks: 1
			<-ch
		}()
	}(0)

	go func() { // main.main.func6
		// main.main.func6.gowrap1
		go func(_ int) { // main.main.func6.1
			// deadlocks: 1
			<-ch
		}(0)
	}()

	// main.main.gowrap8
	go func(_ int) { // main.main.func7
		// main.main.func7.gowrap1
		go func(_ int) { // main.main.func7.1
			// deadlocks: 1
			<-ch
		}(0)
	}(0)

	// main.main.gowrap9
	go func(_ int) { // main.main.func8
		go func() { // main.main.func8.1
			// main.main.func8.1.gowrap1
			go func(_ int) { // main.main.func8.1.1
				// deadlocks: 1
				<-ch
			}(0)
		}()
	}(0)

	go func() { // main.main.func9
		// main.main.func9.gowrap1
		go func(_ int) { // main.main.func9.1
			// deadlocks: 1
			<-ch
		}(0)

		go func() { // main.main.func9.2
			// deadlocks: 1
			<-ch
		}()

		// main.main.func9.gowrap2
		go func(_ int) { // main.main.func9.3
			// deadlocks: 1
			<-ch
		}(0)
	}()

	defer func() { // main.main.func10
		go func() { // main.main.func10.1
			// deadlocks: 1
			<-make(chan int)
		}()
	}()

	defer func() { // main.main.func11
		// main.main.func11.gowrap1
		go func(_ int) { // main.main.func11.1
			// deadlocks: 1
			defer func() {}()
			<-make(chan int)
		}(0)
	}()

	defer func() { // main.main.func12
		// main.main.func12.gowrap1
		go func(_ int) { // main.main.func12.1
			// deadlocks: 1
			defer close(make(chan int))
			go func() { // main.main.func12.1.1
				// deadlocks: 1
				<-make(chan int)
			}()
			<-make(chan int)
		}(0)
	}()

	defer func() { // main.main.func13
		// main.main.func13.gowrap1
		go func(_ int) { // main.main.func13.1
			// deadlocks: 1
			defer foo()
			<-make(chan int)
		}(0)
	}()

	defer func() { // main.main.func14
		// main.main.func14.gowrap1
		go func(_ int) { // main.main.func14.1
			// deadlocks: 1
			defer bar(0)
			<-make(chan int)
		}(0)
	}()

	go func() { // main.main.func15
		// deadlocks: 1
		_ = 0

		// main.main.func14.gowrap1
		defer func(_ int) { // main.main.func15.1
			<-make(chan int)
		}(0)
	}()

	go func() { // main.main.func16
		// deadlocks: 1
		defer func() { // main.main.func16.1
			<-make(chan int)
		}()
	}()

	// main.main.gowrap10
	defer func(_ int) { // main.main.func17
		go func() { // main.main.func17.1
			// deadlocks: 1
			<-make(chan int)
		}()
	}(0)

	// main.main.gowrap11
	defer func(_ int) { // main.main.func18
		// main.main.func18.gowrap1
		go func(_ int) { // main.main.func18.1
			// deadlocks: 1
			defer func() {}()
			<-make(chan int)
		}(0)
	}(0)

	// main.main.gowrap12
	defer func(_ int) { // main.main.func19
		// main.main.func19.gowrap1
		go func(_ int) { // main.main.func19.1
			// deadlocks: 1
			defer close(make(chan int))
			go func() { // main.main.func19.1.1
				// deadlocks: 1
				<-make(chan int)
			}()
			<-make(chan int)
		}(0)
	}(0)

	// main.main.gowrap13
	defer func(_ int) { // main.main.func20
		// main.main.func20.gowrap1
		go func(_ int) { // main.main.func20.1
			// deadlocks: 1
			defer foo()
			<-make(chan int)
		}(0)
	}(0)

	// main.main.gowrap14
	defer func(_ int) { // main.main.func21
		// main.main.func21.gowrap1
		go func(_ int) { // main.main.func21.1
			// deadlocks: 1
			defer bar(0)
			<-make(chan int)
		}(0)
	}(0)

	// main.main.gowrap15
	go func(_ int) { // main.main.func22
		// deadlocks: 1
		<-make(chan int)
	}(0)
}

func foo() {}

func bar(_ int) {}
