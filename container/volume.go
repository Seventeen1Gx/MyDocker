package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

func MountVolume(containerName string, volumeURLs []string) {
	// 宿主机目录
	parentURL := volumeURLs[0]
	if err := os.MkdirAll(parentURL, 0755); err != nil {
		log.Infof("Mkdir dir %s error. %v", parentURL, err)
	}

	// 挂载点
	mntURL := fmt.Sprintf(MountPointURL, containerName)

	// 容器内挂载点：拼接在合并挂载目录 mnt 下
	containerURL := volumeURLs[1]
	containerVolumeURL := filepath.Join(mntURL, containerURL)
	if err := os.MkdirAll(containerVolumeURL, 0755); err != nil {
		log.Infof("Mkdir dir %s error. %v", containerVolumeURL, err)
	}

	// 绑定挂载宿主机目录到容器内路径
	cmd := exec.Command("mount", "--bind", parentURL, containerVolumeURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mount volume failed: %v", err)
	}
}

func DeleteVolumeMountPoint(containerName string, volumeURLs []string) {
	// 卸载数据卷的挂载
	mntURL := fmt.Sprintf(MountPointURL, containerName)
	containerUrl := filepath.Join(mntURL, volumeURLs[1])
	cmd := exec.Command("umount", "-l", containerUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("umount %s failed: %v", containerUrl, err)
	}
}
