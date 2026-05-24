package cgroup_manager

import (
	"fmt"
	"my-docker/subsystem"
	"os"
	"path"
	"strings"
)

type CgroupManager struct {
	Path     string
	Resource *subsystem.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{Path: path}
}

func InitCgroupV2() error {
	root := subsystem.FindCgroupV2MountPoint()
	if root == "" {
		return fmt.Errorf("cgroup v2 not found")
	}
	fmt.Printf("root = %s\n", root)
	// 启用子树控制器
	subtreePath := path.Join(root, "cgroup.subtree_control")
	if _, err := os.Stat(subtreePath); os.IsNotExist(err) {
		return fmt.Errorf("cgroup v2 is not enable correctly")
	}

	controllers := []string{"+memory", "+cpu", "+cpuset"}
	controllerStr := strings.Join(controllers, " ")

	_ = os.WriteFile(subtreePath, []byte(controllerStr), 0644)
	return nil
}

func (c *CgroupManager) Apply(pid int) error {
	for _, subSys := range subsystem.SubsystemIns {
		if err := subSys.Apply(c.Path, pid); err != nil {
			return err
		}
	}
	return nil
}

func (c *CgroupManager) Set() error {
	for _, subSys := range subsystem.SubsystemIns {
		if err := subSys.Set(c.Path, c.Resource); err != nil {
			return err
		}
	}
	return nil
}

func (c *CgroupManager) Destroy() error {
	root := subsystem.FindCgroupV2MountPoint()
	fullPath := path.Join(root, c.Path)
	return os.RemoveAll(fullPath)
}
