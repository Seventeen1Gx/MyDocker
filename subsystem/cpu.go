package subsystem

import (
	"fmt"
	"os"
	"path"
)

type CPUSubsystem struct{}

func (c *CPUSubsystem) Name() string {
	return "cpu"
}

func (c *CPUSubsystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.CpuWeight == "" {
		return nil
	}
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cpu.weight"), []byte(res.CpuWeight), 0644); err != nil {
		return fmt.Errorf("set cgroup cpu share fail: %v", err)
	}
	return nil
}

func (c *CPUSubsystem) Apply(cgroupPath string, pid int) error {
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, false)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cgroup.procs"), []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("apply cgroup cpu share fail: %v", err)
	}
	return nil
}
