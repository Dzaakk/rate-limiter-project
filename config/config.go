package config

import "time"

type ClientConfig struct {
	Limit  int
	Window time.Duration
}

var DefaultConfig = ClientConfig{
	Limit:  100,
	Window: time.Minute,
}

var Clients = map[string]ClientConfig{
	"client-1": {Limit: 5, Window: 60 * time.Second},
	"client-2": {Limit: 2, Window: 60 * time.Second},
}
