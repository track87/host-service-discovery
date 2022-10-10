// Package host_service_discovery declare something
// MarsDong 2022/10/10
package host_service_discovery

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/process"
)

var dockerShimThread = "containerd-shim-runc-v2"
var scriptCheckKernelThread = `
#!/bin/bash

read -a stats < /proc/$1/stat
flags=${stats[8]}

if (( ($flags & 0x00200000) == 0x00200000 )); then
    echo 'YES'
else
    echo 'NO'
fi
`

type Process struct {
	Name           string   `json:"Name"`
	PID            int32    `json:"Pid"`
	PPID           int32    `json:"Ppid"`
	Sockets        []string `json:"Sockets"`
	ExecutablePath string   `json:"ExecutablePath"`
	Args           []string `json:"Args"`
	Ports          []uint32 `json:"Ports"`
	ContainerName  string   `json:"ContainerName"`
	ContainerImage string   `json:"ContainerImage"`
}

type Processes []*Process

type Collector struct {
	//origin  map[int32]*process.Process
	all     Processes
	kernel  []int32
	shim    []int32
	ignored []int32
	valid   []int32
}

func NewCollector() *Collector {
	return &Collector{
		all:     make(Processes, 0),
		kernel:  make([]int32, 0),
		shim:    make([]int32, 0),
		ignored: make([]int32, 0),
		valid:   make([]int32, 0),
	}
}

func (c *Collector) Gen() error {
	pids, err := process.Pids()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		c.all = append(c.all, parseProcess(proc))
	}
	c.initShimThreads()
	if err = c.initKernelThreads(); err != nil {
		return err
	}
	c.initValid()
	return nil
}

func (c *Collector) getProcess(pid int32) *Process {
	for _, p := range c.all {
		if p.PID == pid {
			return p
		}
	}
	return nil
}

func (c *Collector) initKernelThreads() error {
	if err := createCheckScript(scriptCheckKernelThread); err != nil {
		return err
	}
	for _, proc := range c.all {
		if checkKernelThread(proc.PID) {
			c.kernel = append(c.kernel, proc.PID)
		}
	}
	return nil
}

func (c *Collector) initShimThreads() {
	for _, proc := range c.all {
		if proc.Name == dockerShimThread {
			c.shim = append(c.shim, proc.PID)
		}
	}
}

func (c *Collector) initValid() {
	ignoredPids := make([]int32, 0)
	ignoredPids = append(ignoredPids, c.kernel...)
	// filter threads not shim
	ignoredPids = append(ignoredPids, c.getIgnoredShimPids()...)
	ignoredPids = append(ignoredPids, c.getIgnoredSystemPids()...)

	ignoredSet := make(map[int32]struct{})
	for _, p := range ignoredPids {
		ignoredSet[p] = struct{}{}
	}
	for _, proc := range c.all {
		if _, exists := ignoredSet[proc.PID]; !exists {
			c.valid = append(c.valid, proc.PID)
		}
	}
}

func (c *Collector) getIgnoredSystemPids() []int32 {
	ignoredSet := make(map[string]struct{})
	for _, name := range GlobalConf.IgnoredThreads {
		ignoredSet[name] = struct{}{}
	}
	ignored := make([]int32, 0)
	for _, proc := range c.all {
		if _, exists := ignoredSet[proc.Name]; exists {
			ignored = append(ignored, proc.PID)
		}
	}
	return ignored
}

func (c *Collector) getIgnoredShimPids() []int32 {
	roots := make([]*Node, 0)
	for _, pid := range c.shim {
		if proc := c.getProcess(pid); proc != nil {
			roots = append(roots, NewNode(proc.PID, proc.PPID))
		}
	}
	nodes := make([]*Node, 0)
	for _, proc := range c.all {
		node := NewNode(proc.PID, proc.PPID)
		nodes = append(nodes, node)
	}

	ignored := make([]int32, 0)
	tree := NewTree(roots, nodes)
	for _, node := range tree.Traverse() {
		ignored = append(ignored, node.ID)
	}
	return ignored
}

func (c *Collector) String() string {
	str := fmt.Sprintf(`
	Total: %d
	Kernel: %v
	Shim: %v
	Ignored: %v
	Valid: %v
`, len(c.all), c.kernel, c.shim, c.ignored, c.valid)
	return str
}

func checkKernelThread(pid int32) bool {
	cmd := exec.Command("/bin/sh", GlobalConf.KernelThreadCheckScript, strconv.Itoa(int(pid)))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	outputStr := string(output)

	if strings.Contains(outputStr, "YES") {
		return true
	}
	return false
}

func createCheckScript(body string) (err error) {
	var f *os.File
	f, err = os.Create(GlobalConf.KernelThreadCheckScript)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Write([]byte(body))
	return
}

func parseProcess(proc *process.Process) *Process {
	processInfo := &Process{PID: proc.Pid}
	if ports, err := getProcessPorts(proc); err == nil {
		processInfo.Ports = ports
	}
	if name, err := proc.Name(); err == nil {
		processInfo.Name = name
	}
	if executor, err := proc.Exe(); err == nil {
		processInfo.ExecutablePath = executor
	}
	if args, err := proc.CmdlineSlice(); err == nil {
		processInfo.Args = args
	}
	if ppid, err := proc.Ppid(); err == nil {
		processInfo.PPID = ppid
	}
	return processInfo
}

// getProcessSocket
// family: syscall.AF_UNIX, syscall.SOCK_STREAM, syscall.SOCK_DGRAM, syscall.AF_INET, syscall.AF_INET6
// type: syscall.AF_UNIX, syscall.SOCK_STREAM, syscall.SOCK_DGRAM, syscall.AF_INET, syscall.AF_INET6
// @Author MarsDong 2022-10-09 14:46:28
func getProcessPorts(proc *process.Process) ([]uint32, error) {
	var portMap = make(map[uint32]struct{})
	connections, err := proc.Connections()
	if err != nil {
		return nil, err
	}
	for _, conn := range connections {
		if conn.Family != syscall.SOCK_STREAM && conn.Family != syscall.SOCK_DGRAM {
			continue
		}
		if conn.Laddr.Port != 0 {
			portMap[conn.Laddr.Port] = struct{}{}
		}
		if conn.Raddr.Port != 0 {
			portMap[conn.Raddr.Port] = struct{}{}
		}
	}
	ports := make([]uint32, 0)
	for port := range portMap {
		ports = append(ports, port)
	}
	return ports, nil
}
