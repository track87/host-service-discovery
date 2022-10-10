// Package host_service_discovery declare something
// MarsDong 2022/10/10
package host_service_discovery

import (
	"encoding/json"
)

// PrettyObj returns marshal string with indent
func PrettyObj(data interface{}) string {
	p, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return ""
	}
	return string(p)
}
