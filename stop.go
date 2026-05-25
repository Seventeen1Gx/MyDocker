package main

import (
	"encoding/json"
	"fmt"
	"my-docker/container"
	"my-docker/network"
	"os"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func stopContainer(containerName string) {
	// 1. 获取容器信息
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("get container info by name %s error: %v", containerName, err)
		return
	}

	// 2. 发送 SIGTERM 信号终止进程
	pidStr := containerInfo.PID
	pidInt, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Errorf("convert pid %s to int error: %v", pidStr, err)
		return
	}

	if syscall.Kill(pidInt, 0) != nil {
		log.Warnf("container pid %d not found, maybe already stopped", pidInt)
		// 直接跳过 kill，继续更新状态
	} else if err := syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		log.Errorf("kill container pid %d error: %v", pidInt, err)
		return
	}

	// 3. 更新容器状态
	containerInfo.Status = container.STOP
	containerInfo.PID = "" // 清空 PID，标识进程已退出

	// 4. 写回配置文件
	newContentBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("marshal container info error: %v", err)
		return
	}

	dirURL := fmt.Sprintf(container.InfoURL, containerName)
	configFilePath := dirURL + container.ConfigName
	if err := os.WriteFile(configFilePath, newContentBytes, 0622); err != nil {
		log.Errorf("write config file error: %v", err)
	}
}

func removeContainer(containerName string) {
	// 1. 获取容器信息
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("get container info error: %v", err)
		return
	}

	// 2. 只允许删除已停止的容器
	if containerInfo.Status != container.STOP {
		log.Errorf("cannot remove running container, please stop it first")
		return
	}

	// 3. 清理工作
	log.Infof("clean work space")
	network.Disconnect(containerInfo.NetworkName, containerInfo)
	container.DeleteWorkSpace(containerName, containerInfo.Volume)
	deleteContainerInfo(containerName)

	log.Infof("container %s removed successfully", containerName)
}
