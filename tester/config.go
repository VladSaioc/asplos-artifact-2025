package main

import (
	"strings"
)

// Config is a configuration of the program.
type Config []configvalue

var (
	// defaultvalues is the default configuration for the program
	defaultvalues = []Config{{
		gcddtraceOff,
		// gcddtraceOn,
		// gcddtraceTarget,
	}, {
		deadlockDetectionOff,
		deadlockDetectionCollect,
		// deadlockDetectionMonitor,
	}, {
		maxProcs1,
		maxProcs2,
		maxProcs4,
		maxProcs10,
	}}
)

func (c Config) String() string {
	return strings.Join(c.Flags(), " ")
}

func (c Config) Name() string {
	var parts []string
	for _, v := range c {
		parts = append(parts, v.Name())
	}
	return strings.Join(parts, "-")
}

func (c Config) Flags() []string {
	var envVars, gcFlags []string
	for _, v := range c {
		switch v := v.(type) {
		case gcflag:
			gcFlags = append(gcFlags, v.String())
		case configvalue:
			envVars = append(envVars, v.String())
		}
	}

	godebug := "GODEBUG=gctrace=1"
	for _, v := range gcFlags {
		godebug += "," + v
	}
	if perf {
		godebug += ",gcgolfperf=1"
	}
	return append(envVars, godebug)
}

// HasDeadlockDetection returns true if the configuration has deadlock detection enabled.
func (c Config) HasDeadlockDetection() bool {
	for _, v := range c {
		if v, ok := v.(deadlockDetection); ok && (v == deadlockDetectionCollect || v == deadlockDetectionMonitor) {
			return true
		}
	}
	return false
}

// WithToggledDeadlockDetection produces an equivalent configuration,
// except where deadlock detection has been flipped.
func (c Config) WithToggledDeadlockDetection() Config {
	var c2 Config
	for _, v := range c {
		if v, ok := v.(deadlockDetection); ok {
			switch v {
			case deadlockDetectionOff:
				c2 = append(c2, deadlockDetectionCollect)
			case deadlockDetectionCollect:
				c2 = append(c2, deadlockDetectionOff)
			case deadlockDetectionMonitor:
				c2 = append(c2, deadlockDetectionOff)
			}
			continue
		}
		c2 = append(c2, v)
	}
	return c2
}

// EmitConfigurations produces configurations that should be run on the given examples.
func EmitConfigurations() chan Config {
	ch := make(chan Config, 10)
	go func() {
		var constructConfig func([]configvalue, int)
		constructConfig = func(c []configvalue, i int) {
			if i == len(defaultvalues) {
				ch <- c
				return
			}

			for _, v := range defaultvalues[i] {
				cpy := make([]configvalue, len(c)+1)
				copy(cpy, append(c, v))
				constructConfig(cpy, i+1)
			}
		}

		constructConfig(nil, 0)
		close(ch)
	}()
	return ch
}
