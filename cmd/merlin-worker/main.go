// Command merlin-worker runs the agent runtime and the durable workflow workers.
// Every runtime role executes here as configuration on one reusable runtime;
// roles are configuration, not servers (deck slides 8, 10, 15).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "merlin-worker: not implemented yet (Phase 0)")
	os.Exit(1)
}
