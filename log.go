package main

import (
	"fmt"
	"io"
	"my-docker/container"
	"os"

	log "github.com/sirupsen/logrus"
)

func logContainer(containerName string) {
	dirURL := fmt.Sprintf(container.InfoURL, containerName)
	logFileLocation := dirURL + container.ContainerLogFile

	// 打开日志文件
	file, err := os.Open(logFileLocation)
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFileLocation, err)
		return
	}
	defer file.Close()

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFileLocation, err)
		return
	}

	// 打印日志到标准输出
	fmt.Fprint(os.Stdout, string(content))
}
