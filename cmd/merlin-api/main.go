// Command merlin-api runs the MerlinCI control plane: the Go API that holds
// configuration and permissions, exposes runs and artifacts for inspection, and
// lets a person pause, inspect and revoke autonomy (deck slides 6, 10, 11, 15).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "merlin-api: not implemented yet (Phase 0)")
	os.Exit(1)
}
