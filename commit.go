package main

import (
	"fmt"
	"my-docker/container"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func commitContainer(imageName, containerName string) {
	mntURL := fmt.Sprintf(container.MountPointURL, containerName)
	imageTar := container.ImageURL + "/" + imageName + ".tar"

	cmd := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".")
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Errorf("commit container error: %v", err)
		return
	}

	log.Infof("commit container success, image tar is %s", imageTar)
}
