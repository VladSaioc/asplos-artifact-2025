package main

import "fmt"

type (
	// maxProcs is the type to represent the GOMAXPROCS value
	maxProcs int
	// deadlockDetection is the type to represent the `gcdetectdeadlocks` flag value
	deadlockDetection int
	// gcddtrace is the type to represent the `gcddtrace` flag value
	gcddtrace int

	// configvalue should be implemented by all configuration value types
	configvalue interface {
		Name() string
		String() string
		isConfigValue()
	}

	// gcflag must be implemented by all GC flag types.
	gcflag interface {
		String() string
		isGCFlag()
	}
)

const (
	// Values for the maxProcs type: 1, 10
	maxProcs1, maxProcs2, maxProcs4, maxProcs10 maxProcs = 1, 2, 4, 10

	// Values for the deadlockDetection type: 0, 1, 2
	deadlockDetectionOff, deadlockDetectionCollect, deadlockDetectionMonitor deadlockDetection = 0, 1, 2

	// Values for the gcddtrace type: 0, 1
	gcddtraceOff, gcddtraceOn, gcddtraceTarget gcddtrace = 0, 1, 2
)

func (m maxProcs) String() string {
	switch m {
	case maxProcs1, maxProcs2, maxProcs4, maxProcs10:
		return fmt.Sprintf("GOMAXPROCS=%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized GOMAXPROCS value: %v", int(m)))
}

func (m maxProcs) Name() string {
	switch m {
	case maxProcs1, maxProcs2, maxProcs4, maxProcs10:
		return fmt.Sprintf("GOMAXPROCS-%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized GOMAXPROCS value: %v", int(m)))
}

func (m maxProcs) isConfigValue() {}

func (m deadlockDetection) String() string {
	switch m {
	case deadlockDetectionOff,
		deadlockDetectionCollect,
		deadlockDetectionMonitor:
		return fmt.Sprintf("gcdetectdeadlocks=%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized gcdetectdeadlocks value: %v", int(m)))
}

func (m deadlockDetection) Name() string {
	switch m {
	case deadlockDetectionOff,
		deadlockDetectionCollect,
		deadlockDetectionMonitor:
		return fmt.Sprintf("gcdetectdeadlocks-%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized gcdetectdeadlocks value: %v", int(m)))
}

func (m deadlockDetection) isGCFlag()      {}
func (m deadlockDetection) isConfigValue() {}

func (m gcddtrace) String() string {
	switch m {
	case gcddtraceOff, gcddtraceOn, gcddtraceTarget:
		return fmt.Sprintf("gcddtrace=%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized gcddtrace value: %v", int(m)))
}

func (m gcddtrace) Name() string {
	switch m {
	case gcddtraceOff, gcddtraceOn, gcddtraceTarget:
		return fmt.Sprintf("gcddtrace-%v", int(m))
	}
	panic(fmt.Sprintf("Unrecognized gcddtrace value: %v", int(m)))
}

func (m gcddtrace) isGCFlag()      {}
func (m gcddtrace) isConfigValue() {}
