//go:build !linux && !darwin

package hardware

// memTotalMiB is unknown on unsupported platforms.
func memTotalMiB() int64 { return 0 }
