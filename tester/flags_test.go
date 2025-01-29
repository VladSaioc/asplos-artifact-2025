package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagMaxProcsString(t *testing.T) {
	require.Equal(t, "GOMAXPROCS=1", maxProcs1.String())
	require.PanicsWithValue(t, "Unrecognized GOMAXPROCS value: 0", func() {
		_ = maxProcs(0).String()
	})
}

func TestFlagMaxProcsName(t *testing.T) {
	require.Equal(t, "GOMAXPROCS-1", maxProcs1.Name())
	require.PanicsWithValue(t, "Unrecognized GOMAXPROCS value: 0", func() {
		maxProcs(0).isConfigValue()
		_ = maxProcs(0).Name()
	})
}

func TestFlagDeadlockDetectionString(t *testing.T) {
	deadlockDetectionOff.isGCFlag()

	require.Equal(t, "gcdetectdeadlocks=0", deadlockDetectionOff.String())
	require.PanicsWithValue(t, "Unrecognized gcdetectdeadlocks value: -1", func() {
		_ = deadlockDetection(-1).String()
	})
}

func TestFlagDeadlockDetectionName(t *testing.T) {
	deadlockDetectionOff.isGCFlag()
	deadlockDetectionOff.isConfigValue()

	require.Equal(t, "gcdetectdeadlocks-0", deadlockDetectionOff.Name())
	require.PanicsWithValue(t, "Unrecognized gcdetectdeadlocks value: -1", func() {
		_ = deadlockDetection(-1).Name()
	})
}

func TestFlagDeadlockDetectionDebugTraceString(t *testing.T) {
	gcddtraceOff.isGCFlag()
	gcddtraceOff.isConfigValue()

	require.Equal(t, "gcddtrace=0", gcddtraceOff.String())
	require.PanicsWithValue(t, "Unrecognized gcddtrace value: -1", func() {
		_ = gcddtrace(-1).String()
	})
}

func TestFlagDeadlockDetectionDebugTraceName(t *testing.T) {
	gcddtraceOff.isGCFlag()
	gcddtraceOff.isConfigValue()

	require.Equal(t, "gcddtrace-0", gcddtraceOff.Name())
	require.PanicsWithValue(t, "Unrecognized gcddtrace value: -1", func() {
		_ = gcddtrace(-1).Name()
	})
}
