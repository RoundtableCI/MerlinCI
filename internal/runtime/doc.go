// Package runtime is the single reusable agent runtime. The model proposes;
// MerlinCI controls execution (deck slide 10). The agent loop:
//
//	01 Load configuration and permissions
//	02 Retrieve campaign and company context
//	03 Request inference through the gateway
//	04 Validate and run only permitted tool calls
//	05 Checkpoint state, then continue or stop
//	06 Validate output, store the artifact
package runtime
