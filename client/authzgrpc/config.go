package authzgrpc

import (
	"strings"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
)

// Config describes the IAM authorization gRPC downstream.
type Config struct {
	Endpoint       string           `json:"endpoint" yaml:"endpoint"`
	CallerService  string           `json:"caller_service" yaml:"caller_service"`
	Insecure       bool             `json:"insecure" yaml:"insecure"`
	Timeout        time.Duration    `json:"timeout" yaml:"timeout"`
	RetryMax       uint             `json:"retry_max" yaml:"retry_max"`
	MetricsEnabled bool             `json:"metrics_enabled" yaml:"metrics_enabled"`
	Logger         logx.Logger      `json:"-" yaml:"-"`
	Metrics        metricsx.Manager `json:"-" yaml:"-"`
}

func (c Config) normalize() Config {
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.CallerService = strings.TrimSpace(c.CallerService)
	if c.Timeout <= 0 {
		c.Timeout = 3 * time.Second
	}
	return c
}
