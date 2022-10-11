// Package host_service_discovery declare something
// MarsDong 2022/10/10
package host_service_discovery

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

const (
	ModeMatch = "match"
	ModeRegex = "regex"
)

var GlobalConf = &Config{}

// Config configuration for service discovery
type Config struct {
	IgnoredThreads          []string `json:"IgnoredThreads" yaml:"ignoredThreads"`
	KernelThreadCheckScript string   `json:"KernelThreadCheckScript" yaml:"kernelThreadCheckScript"`
	ShimThread              string   `json:"ShimThread" yaml:"shimThread"`
}

func MustInitConf(confFile string) {
	if err := InitConf(confFile); err != nil {
		panic(err)
	}
}

func InitConf(confFile string) error {
	contents, err := ioutil.ReadFile(confFile)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(contents, GlobalConf); err != nil {
		return err
	}
	return nil
}
