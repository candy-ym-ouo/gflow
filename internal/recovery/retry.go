package recovery

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type Policy struct {
	MaxAttempts int
	Interval    time.Duration
	Backoff     string
}

func (p Policy) Next(attempt int) time.Time {
	d := p.Delay(attempt)
	return time.Now().Add(d)
}
func (p Policy) Delay(attempt int) time.Duration {
	if p.Interval <= 0 {
		p.Interval = time.Second
	}
	if attempt < 1 {
		attempt = 1
	}
	d := p.Interval
	switch strings.ToLower(p.Backoff) {
	case "exponential":
		d = time.Duration(float64(d) * math.Pow(2, float64(attempt-1)))
	case "linear":
		d = d * time.Duration(attempt)
	}
	return d
}
func (p Policy) Valid() error {
	if p.MaxAttempts < 1 {
		return errors.New("max attempts must be positive")
	}
	if p.Interval <= 0 {
		return errors.New("retry interval must be positive")
	}
	switch strings.ToLower(p.Backoff) {
	case "fixed", "linear", "exponential", "":
	default:
		return fmt.Errorf("unsupported backoff %s", p.Backoff)
	}
	return nil
}
func (p Policy) Exhausted(attempt int) bool { return attempt >= p.MaxAttempts }

type Class string

const (
	RetryableClass Class = "retryable"
	Fatal          Class = "fatal"
	Timeout        Class = "timeout"
)

func Classify(err error) Class {
	if err == nil {
		return ""
	}
	if errors.Is(err, contextDeadline()) {
		return Timeout
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return Timeout
	}
	if strings.Contains(strings.ToLower(err.Error()), "invalid") {
		return Fatal
	}
	return RetryableClass
}

func contextDeadline() error { return errors.New("context deadline exceeded") }
