package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"my-docker/cgroup_manager"
	"my-docker/container"
	"my-docker/network"
	"my-docker/subsystem"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func Run(tty bool, comArray []string, res *subsystem.ResourceConfig, imageName, containerName, volume string,
	envArray []string, nw string, portmapping []string) {
	containerID := randStringBytes(10)
	if containerName == "" {
		containerName = containerID
	}

	parent, writePipe := container.NewParentProcess(tty, imageName, containerName, volume, envArray)
	if err := parent.Start(); err != nil {
		log.Errorf("start error: %v", err)
		return
	}

	command := strings.Join(comArray, " ")
	containerInfo := &container.ContainerInfo{
		ID:          containerID,
		PID:         strconv.Itoa(parent.Process.Pid),
		Name:        containerName,
		Command:     command,
		CreatedTime: time.Now().Format("2006-01-02 15:04:05"),
		Status:      container.RUNNING,
		Volume:      volume,
		NetworkName: nw,
		PortMapping: portmapping,
	}

	if nw != "" {
		network.Init()
		if err := network.Connect(nw, containerInfo); err != nil {
			log.Errorf("Error Connect Network %v", err)
			return
		}
	}

	// 记录容器信息
	err := recordContainerInfo(containerInfo)
	if err != nil {
		log.Errorf("recordContainerInfo: %v", err)
		return
	}

	// 资源限制
	if res != nil {
		cgroupManager := cgroup_manager.NewCgroupManager("my-demo-docker")
		cgroupManager.Resource = res
		defer cgroupManager.Destroy()
		if err := cgroupManager.Set(); err != nil {
			log.Error(err.Error())
			return
		}
		if err := cgroupManager.Apply(parent.Process.Pid); err != nil {
			log.Error(err.Error())
			return
		}
	}

	sendInitCommand(command, writePipe)

	if tty {
		if err := parent.Wait(); err != nil {
			log.Errorf("wait error: %v", err)
		}

		// 清理工作
		log.Infof("clean work space")
		network.Disconnect(nw, containerInfo)
		container.DeleteWorkSpace(containerName, volume)
		deleteContainerInfo(containerName)
	}

	// 后台模式不等待子进程结束，直接返回
}

func sendInitCommand(command string, writePipe *os.File) {
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

func randStringBytes(n int) string {
	letterBytes := "1234567890"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func recordContainerInfo(containerInfo *container.ContainerInfo) error {
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error: %v", err)
		return err
	}
	jsonStr := string(jsonBytes)

	dirUrl := fmt.Sprintf(container.InfoURL, containerInfo.Name)
	if err := os.MkdirAll(dirUrl, 0755); err != nil {
		log.Errorf("Mkdir error %s error %v", dirUrl, err)
		return err
	}
	// /home/gxx/my-docker-test/info/容器名/config.json
	fileName := dirUrl + container.ConfigName
	file, err := os.Create(fileName)
	if err != nil {
		log.Errorf("Create file %s error %v", fileName, err)
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error %v", err)
		return err
	}

	return nil
}

func deleteContainerInfo(containerName string) {
	dirURL := fmt.Sprintf(container.InfoURL, containerName)
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove dir %s error %v", dirURL, err)
	}
}
