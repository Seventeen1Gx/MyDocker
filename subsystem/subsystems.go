package subsystem

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

type ResourceConfig struct {
	MemoryMax string
	CpuWeight string
	CpuSet    string
}

type Subsystem interface {
	Name() string
	Set(cgroupPath string, res *ResourceConfig) error
	Apply(cgroupPath string, pid int) error
}

var SubsystemIns = []Subsystem{
	&MemorySubsystem{},
	&CPUSubsystem{},
	&CPUSetSubsystem{},
}

func FindCgroupV2MountPoint() string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	// 120 111 0:23 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:23 - cgroup2 cgroup2 rw,nsdelegate
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, " ")
		if len(fields) < 6 {
			continue
		}
		for i := 0; i < len(fields); i++ {
			if fields[i] == "-" && i+1 < len(fields) && fields[i+1] == "cgroup2" {
				if i-3 >= 0 {
					return fields[i-3]
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ""
	}

	// 默认
	return "/sys/fs/cgroup"
}

func GetCgroupV2Path(cgroupPath string, autoCreate bool) (string, error) {
	root := FindCgroupV2MountPoint()
	if root == "" {
		return "", fmt.Errorf("cgroup v2 mountpoint not found")
	}

	fullPath := path.Join(root, cgroupPath)

	if autoCreate {
		if _, err := os.Stat(fullPath); err != nil && os.IsNotExist(err) {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return "", fmt.Errorf("create cgroup path error: %v", err)
			}
			log.Infof("create cgroup path %s success", fullPath)
		}
	}

	return fullPath, nil
}
