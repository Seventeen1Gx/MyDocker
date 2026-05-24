package container

import (
	"fmt"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// 从指定镜像创建只读层（共享）
func CreateReadOnlyLayer(imageName string) {
	// 下载的镜像 tar 包在这个目录下
	imageTarURL := ImageURL + "/" + imageName + ".tar"
	// 解压到镜像对应的只读层
	unTarFolderURL := fmt.Sprintf(LowerLayerURL, imageName)
	if _, err := os.Stat(unTarFolderURL); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(unTarFolderURL, 0755); err != nil {
			log.Errorf("Mkdir dir %s error. %v", unTarFolderURL, err)
		}
		if _, err := exec.Command("tar", "-xvf", imageTarURL, "-C", unTarFolderURL).CombinedOutput(); err != nil {
			log.Errorf("unTar %s error %v", imageTarURL, err)
		}
	}
}

// 从指定容器创建可写层
func CreateWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(UpperLayerURL, containerName)
	if err := os.MkdirAll(writeURL, 0755); err != nil {
		log.Infof("Mkdir dir %s error. %v", writeURL, err)
	}
}

// 从指定容器创建 work 目录
func CreateWorkDir(containerName string) {
	workURL := fmt.Sprintf(WorkDirURL, containerName)
	if err := os.MkdirAll(workURL, 0755); err != nil {
		log.Infof("Mkdir dir %s error. %v", workURL, err)
	}
}

// 挂载 OverlayFS
func CreateMountPoint(imageName, containerName string) {
	mntURL := fmt.Sprintf(MountPointURL, containerName)

	// 先清空旧环境
	_ = exec.Command("umount", mntURL).Run()
	_ = os.RemoveAll(mntURL)

	// 创建 mnt 目录作为挂载点
	if err := os.MkdirAll(mntURL, 0755); err != nil {
		log.Infof("Mkdir dir %s error. %v", mntURL, err)
	}

	lowerDir := fmt.Sprintf(LowerLayerURL, imageName)
	upperDir := fmt.Sprintf(UpperLayerURL, containerName)
	workDir := fmt.Sprintf(WorkDirURL, containerName)

	// OverlayFS 挂载参数：lowerdir(只读层):upperdir(可写层):workdir(工作目录)
	dirs := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		lowerDir,
		upperDir,
		workDir)

	log.Infof("Mounting overlay filesystem with dirs: %s", dirs)

	cmd := exec.Command("mount", "-t", "overlay", "-o", dirs, "overlay", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mount overlay failed: %v", err)
	}
}

func DeleteMountPoint(containerName string) {
	mntURL := fmt.Sprintf(MountPointURL, containerName)
	// 先卸载 mnt 目录
	cmd := exec.Command("umount", "-l", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("umount %s failed: %v", mntURL, err)
	}
	// 删除 mnt 目录
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove dir %s error %v", mntURL, err)
	}
}

func DeleteLayer(containerName string) {
	layerURL := fmt.Sprintf(LayerURL, containerName)
	if err := os.RemoveAll(layerURL); err != nil {
		log.Errorf("Remove dir %s error %v", layerURL, err)
	}
}
