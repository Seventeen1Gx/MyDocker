package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	_ "my-docker/nsenter"

	log "github.com/sirupsen/logrus"
)

const (
	ENV_EXEC_PID = "mydocker_pid"
	ENV_EXEC_CMD = "mydocker_cmd"
)

func ExecContainer(containerName string, comArray []string) {
	// 1. 获取容器 PID
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		log.Errorf("Get container pid by name %s error: %v", containerName, err)
		return
	}

	// 2. 拼接命令
	cmdStr := strings.Join(comArray, " ")
	log.Infof("container pid %s", pid)
	log.Infof("command %s", cmdStr)

	// 3. 设置环境变量，传递 PID 和命令
	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, cmdStr)

	// 4. 调用 /proc/self/exe 重新执行自身，触发 Cgo 代码
	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	containerEnvs := getEnvsByPid(pid)
	cmd.Env = append(os.Environ(), containerEnvs...)

	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container %s error: %v", containerName, err)
	}
}

func getEnvsByPid(pid string) []string {
	// 进程环境变量存在 /proc/<pid>/environ 中，用 \0 分隔
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("read environ file %s error: %v", path, err)
		return nil
	}
	// 按 \0 分割成环境变量数组
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs
}
