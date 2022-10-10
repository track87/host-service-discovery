// Package main entry
// MarsDong 2022/10/10
package main

import (
	"fmt"

	"github.com/track87/host-service-discovery"
)

func main() {
	host_service_discovery.MustInitConf("./config.yaml")
	collector := host_service_discovery.NewCollector()
	if err := collector.Gen(); err != nil {
		panic(err)
	}
	fmt.Println(collector.String())
	processes := collector.GetValidProcess()
	fmt.Println(host_service_discovery.PrettyObj(processes))
}
