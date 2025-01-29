package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigFlags(t *testing.T) {
	c := Config{
		maxProcs1,
		gcddtraceOn,
	}
	require.EqualValues(t, []string{
		"GOMAXPROCS=1",
		"GODEBUG=gctrace=1,gcddtrace=1",
	}, c.Flags())
}

func TestConfigName(t *testing.T) {
	c := Config{
		maxProcs1,
		gcddtraceOn,
	}
	require.Equal(t, "GOMAXPROCS-1-gcddtrace-1", c.Name())
}

func TestConfigString(t *testing.T) {
	c := Config{
		maxProcs1,
		gcddtraceOn,
	}
	require.Equal(t, "GOMAXPROCS=1 GODEBUG=gctrace=1,gcddtrace=1", c.String())
}

func TestConfigHasDeadlockDetection(t *testing.T) {
	c := Config{
		maxProcs1,
		deadlockDetectionCollect,
	}
	require.True(t, c.HasDeadlockDetection())

	c = Config{maxProcs1}
	require.False(t, c.HasDeadlockDetection())
}

func TestEmitConfigurations(t *testing.T) {
	configs := EmitConfigurations()

	yieldedConfigs := 0
	for range configs {
		yieldedConfigs++
	}

	expectedConfigs := 1
	for _, vs := range defaultvalues {
		expectedConfigs *= len(vs)
	}

	require.Equal(t, expectedConfigs, yieldedConfigs)
}
