package retrying

import "time"

// Backoff calcula tempo de retry com jitter determinístico
func Backoff(attempts int, base time.Duration, factor float64, jitter float64) time.Duration {
	jitterDuration := time.Duration(float64(base.Nanoseconds()) * jitter)
	return time.Duration(float64(base.Nanoseconds())*factor) + jitterDuration
}
