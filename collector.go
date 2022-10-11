// Package host_service_discovery declare something
// MarsDong 2022/10/10
package host_service_discovery

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
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
	all     map[int32]*Process
	kernel  []int32
	shim    []int32
	ignored []int32
	valid   []int32

	ignoreRegex []*regexp.Regexp
}

func NewCollector() *Collector {
	c := &Collector{
		all:         make(map[int32]*Process, 0),
		kernel:      make([]int32, 0),
		shim:        make([]int32, 0),
		ignored:     make([]int32, 0),
		valid:       make([]int32, 0),
		ignoreRegex: make([]*regexp.Regexp, 0),
	}

	for _, expression := range GlobalConf.IgnoredThreads {
		c.ignoreRegex = append(c.ignoreRegex, regexp.MustCompile(expression))
	}
	return c
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
		c.all[pid] = parseProcess(proc)
	}
	c.initShimThreads()
	if err = c.initKernelThreads(); err != nil {
		return err
	}
	c.initValid()
	return nil
}

func (c *Collector) initKernelThreads() error {
	if err := createCheckScript(scriptCheckKernelThread); err != nil {
		return err
	}
	for pid := range c.all {
		if checkKernelThread(pid) {
			c.kernel = append(c.kernel, pid)
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
	c.ignored = ignoredPids

	ignoredSet := make(map[int32]struct{})
	for _, p := range ignoredPids {
		ignoredSet[p] = struct{}{}
	}
	for pid := range c.all {
		if _, exists := ignoredSet[pid]; !exists {
			c.valid = append(c.valid, pid)
		}
	}
}

func (c *Collector) getIgnoredSystemPids() []int32 {
	ignored := make([]int32, 0)
	for _, proc := range c.all {
		for _, reg := range c.ignoreRegex {
			if reg.MatchString(proc.Name) {
				ignored = append(ignored, proc.PID)
				break
			}
		}
	}
	return ignored
}

func (c *Collector) getIgnoredShimPids() []int32 {
	roots := make([]*Node, 0)
	for _, pid := range c.shim {
		if v, exists := c.all[pid]; exists {
			roots = append(roots, NewNode(v.PID, v.PPID))
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
	kernelStr := strings.Join(c.getKernelThreadNames(), ", ")
	shimStr := strings.Join(c.getShimThreadNames(), ", ")
	ignoredStr := strings.Join(c.getIgnoredThreadNames(), ", ")
	validStr := strings.Join(c.getValidThreadNames(), ", ")

	str := fmt.Sprintf(`
	Total: %d
	Kernel: %s
	Shim: %s
	Ignored: %s
	Valid: %s
`, len(c.all), kernelStr, shimStr, ignoredStr, validStr)
	return str
}

func (c *Collector) GetValidProcess() Processes {
	processes := make(Processes, 0)
	for _, pid := range c.valid {
		processes = append(processes, c.all[pid])
	}
	return processes
}

func (c *Collector) getShimThreadNames() []string {
	names := make([]string, 0)
	for _, pid := range c.shim {
		names = append(names, c.all[pid].Name)
	}
	return names
}

func (c *Collector) getKernelThreadNames() []string {
	names := make([]string, 0)
	for _, pid := range c.kernel {
		names = append(names, c.all[pid].Name)
	}
	return names
}

func (c *Collector) getIgnoredThreadNames() []string {
	names := make([]string, 0)
	for _, pid := range c.ignored {
		names = append(names, c.all[pid].Name)
	}
	return names
}

func (c *Collector) getValidThreadNames() []string {
	names := make([]string, 0)
	for _, pid := range c.valid {
		names = append(names, c.all[pid].Name)
	}
	return names
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
