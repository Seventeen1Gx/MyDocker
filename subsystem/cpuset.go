package subsystem

import (
	"fmt"
	"os"
	"path"
)

type CPUSetSubsystem struct{}

func (c *CPUSetSubsystem) Name() string {
	return "cpuset"
}

func (c *CPUSetSubsystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.CpuSet == "" {
		return nil
	}
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cpuset.cpus"), []byte(res.CpuSet), 0644); err != nil {
		return fmt.Errorf("set cgroup cpuset fail: %v", err)
	}
	return nil
}

func (c *CPUSetSubsystem) Apply(cgroupPath string, pid int) error {
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, false)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cgroup.procs"), []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("apply cgroup cpuset fail: %v", err)
	}
	return nil
}
