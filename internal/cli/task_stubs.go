package cli

import "os"

// task_stubs.go provides var-based seams for task.go error-path testing.

var (
	osReadFileFn  = os.ReadFile
	osWriteFileFn = os.WriteFile
)
