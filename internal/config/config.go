package config

import (
	"flag"
	"os"
	"time"
)

type Config struct {
	Addr     string
	Interval time.Duration
	Workers  int
}

func Default() Config { return Config{Addr: ":8080", Interval: time.Second, Workers: 2} }
func Load() Config {
	c := Default()
	if v := os.Getenv("GFLOW_ADDR"); v != "" {
		c.Addr = v
	}
	return c
}
func Flags(c *Config) {
	flag.StringVar(&c.Addr, "addr", c.Addr, "listen address")
	flag.DurationVar(&c.Interval, "interval", c.Interval, "scheduler interval")
	flag.IntVar(&c.Workers, "workers", c.Workers, "worker count")
}
