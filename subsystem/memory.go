package subsystem

import (
	"fmt"
	"os"
	"path"
)

type MemorySubsystem struct{}

func (m *MemorySubsystem) Name() string {
	return "memory"
}

func (m *MemorySubsystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.MemoryMax == "" {
		return nil
	}
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, true)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "memory.max"), []byte(res.MemoryMax), 0644); err != nil {
		return fmt.Errorf("set cgroup memory fail: %v", err)
	}
	return nil
}

func (m *MemorySubsystem) Apply(cgroupPath string, pid int) error {
	subsysCgroupPath, err := GetCgroupV2Path(cgroupPath, false)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "cgroup.procs"), []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("apply cgroup memory fail: %v", err)
	}
	return nil
}
