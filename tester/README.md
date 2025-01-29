# Testing framework for GC-based Partial Deadlock detection

This framework accepts a Go binary, and drives the running of the Go compiler and runtime
in various configurations. It validates GC traces emited by the runtime under each configuration,
by ensuring that e.g., the runtime does not panic, or deadlocks are not incorrectly reported.

## Run-time configuration

The framework tests all combinations of the following configuration keys and values:
  * Deadlock detection (`gcdetectdeadlocks`) may be:
    - Disabled (`0`)
    - Enabled for garbage collection (`1`)
    - Enabled only for monitoring (`2`)
  * Maximum logical processors (`GOMAXPROCS`): `1`, `2` and `10`
  * Stop the world during GC (`gcstoptheworld`): disabled (`0`), enabled when marking (`1`), enabled for all GC steps (`2`)
