package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

func (d *Redis) UnmarshalYAML(node *yaml.Node) error {
	var r struct {
		Addr           string `yaml:"addr"`
		Username       string `yaml:"username"`
		PasswordEnv    string `yaml:"password_env"`
		TLS            bool   `yaml:"tls"`
		InsecureTLS    bool   `yaml:"insecure_tls"`
		ConnectTimeout string `yaml:"connect_timeout"`
		CommandTimeout string `yaml:"command_timeout"`
	}
	if err := node.Decode(&r); err != nil {
		return err
	}
	if r.Addr != "" {
		d.Addr = r.Addr
	}
	if r.Username != "" {
		d.Username = r.Username
	}
	if r.PasswordEnv != "" {
		d.PasswordEnv = r.PasswordEnv
	}
	d.TLS, d.InsecureTLS = r.TLS, r.InsecureTLS
	var err error
	if r.ConnectTimeout != "" {
		d.ConnectTimeout, err = time.ParseDuration(r.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("connect_timeout: %w", err)
		}
	}
	if r.CommandTimeout != "" {
		d.CommandTimeout, err = time.ParseDuration(r.CommandTimeout)
		if err != nil {
			return fmt.Errorf("command_timeout: %w", err)
		}
	}
	return nil
}

func (s *Sampling) UnmarshalYAML(node *yaml.Node) error {
	var r struct {
		Duration   string `yaml:"duration"`
		Interval   string `yaml:"interval"`
		ScanCount  int64  `yaml:"scan_count"`
		SampleSize int    `yaml:"sample_size"`
		ScanRate   int    `yaml:"scan_rate"`
		Top        int    `yaml:"top"`
	}
	if err := node.Decode(&r); err != nil {
		return err
	}
	var err error
	if r.Duration != "" {
		s.Duration, err = time.ParseDuration(r.Duration)
		if err != nil {
			return fmt.Errorf("duration: %w", err)
		}
	}
	if r.Interval != "" {
		s.Interval, err = time.ParseDuration(r.Interval)
		if err != nil {
			return fmt.Errorf("interval: %w", err)
		}
	}
	if r.ScanCount != 0 {
		s.ScanCount = r.ScanCount
	}
	if r.SampleSize != 0 {
		s.SampleSize = r.SampleSize
	}
	if r.ScanRate != 0 {
		s.ScanRate = r.ScanRate
	}
	if r.Top != 0 {
		s.Top = r.Top
	}
	return nil
}
