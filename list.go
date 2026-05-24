package main

import (
	"encoding/json"
	"fmt"
	"my-docker/container"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
)

func ListContainers() {
	dirURL := fmt.Sprintf(container.InfoURL, "")
	dirURL = dirURL[:len(dirURL)-1] // 去掉末尾的 "/"

	// 读取目录下所有容器文件夹
	files, err := os.ReadDir(dirURL)
	if err != nil {
		log.Errorf("Read dir %s error %v", dirURL, err)
		return
	}

	var containers []*container.ContainerInfo
	// 遍历每个容器目录，读取 config.json
	for _, file := range files {
		tmpContainer, err := getContainerInfoByName(file.Name())
		if err != nil {
			log.Errorf("Get container info %s error %v", file.Name(), err)
			continue
		}
		containers = append(containers, tmpContainer)
	}

	// 使用 tabwriter 格式化输出表格
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// 打印表头
	fmt.Fprintf(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")
	// 打印每个容器信息
	for _, item := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Name,
			item.PID,
			item.Status,
			item.Command,
			item.CreatedTime)
	}
	// 刷新缓冲区，输出表格
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
	}
}

// getContainerInfoByName 根据容器名获取完整信息
func getContainerInfoByName(containerName string) (*container.ContainerInfo, error) {
	dirURL := fmt.Sprintf(container.InfoURL, containerName)
	configFilePath := dirURL + container.ConfigName
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return nil, err
	}
	return &containerInfo, nil
}

// getContainerPidByName 只获取容器 PID
func getContainerPidByName(containerName string) (string, error) {
	info, err := getContainerInfoByName(containerName)
	if err != nil {
		return "", err
	}
	return info.PID, nil
}
