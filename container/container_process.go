package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var (
	RUNNING = "running"
	STOP    = "stopped"
	Exit    = "exited"
)

var (
	RootURL          string = "/home/gxx/my-docker-test" // 根工作目录
	InfoURL          string = RootURL + "/info/%s/"      // 容器信息存放目录
	ImageURL         string = RootURL + "/images"        // 镜像存放目录（tar包）
	LayerURL         string = RootURL + "/layers/%s"     // 镜像和容器层存放目录
	LowerLayerURL    string = LayerURL + "/lower"        // 镜像只读层（每个镜像独立）
	UpperLayerURL    string = LayerURL + "/upper"        // 容器读写层（每个容器独立）
	WorkDirURL       string = LayerURL + "/work"         // OverlayFS 工作目录（每个容器独立）
	MountPointURL    string = RootURL + "/mnt/%s"        // 容器挂载点（每个容器独立）
	ConfigName              = "config.json"              // 存放容器信息
	ContainerLogFile        = "container.log"            // 容器日志文件
)

type ContainerInfo struct {
	PID         string   `json:"pid"`         // 容器启动进程在宿主机上的 PID
	ID          string   `json:"id"`          // 容器唯一标识
	Name        string   `json:"name"`        // 容器名称
	Command     string   `json:"command"`     // 容器内运行的命令
	CreatedTime string   `json:"createTime"`  // 容器创建时间
	Status      string   `json:"status"`      // 容器状态
	Volume      string   `json:"volume"`      // 容器挂载的卷信息
	NetworkName string   `json:"networkName"` // 容器连接的网络名
	IP          string   `json:"ip"`          // 容器 IP 地址
	PortMapping []string `json:"portMapping"` // 端口映射
}

// 父进程与子进程之间通过管道通信
// 父进程将用户输入的命令发送给子进程
// 子进程在隔离环境中执行该命令
func NewParentProcess(tty bool, imageName, containerName, volume string, envArray []string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := NewPipe()
	if err != nil {
		log.Errorf("NewPipe error: %v", err)
		return nil, nil
	}

	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 非交互模式，日志输出到文件
		dirURL := fmt.Sprintf(InfoURL, containerName)
		if err := os.MkdirAll(dirURL, 0755); err != nil {
			log.Errorf("NewParentProcess mkdir %s error %v", dirURL, err)
			return nil, nil
		}
		// 日志文件路径：/home/gxx/my-docker-test/info/容器名/container.log
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParentProcess create file %s error %v", stdLogFilePath, err)
			return nil, nil
		}
		// 重定向 stdout 和 stderr 到日志文件
		cmd.Stdout = stdLogFile
		cmd.Stderr = stdLogFile
	}

	// 继承宿主机环境变量，并追加用户自定义的变量
	cmd.Env = append(os.Environ(), envArray...)

	cmd.ExtraFiles = []*os.File{readPipe}

	NewWorkSpace(imageName, containerName, volume)
	cmd.Dir = fmt.Sprintf(MountPointURL, containerName)

	return cmd, writePipe
}

func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}

func NewWorkSpace(imageName, containerName, volume string) {
	CreateReadOnlyLayer(imageName)
	CreateWriteLayer(containerName)
	CreateWorkDir(containerName)
	CreateMountPoint(imageName, containerName)
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(containerName, volumeURLs)
		} else {
			log.Errorf("Volume parameter error: %s", volume)
		}
	}
}

func DeleteWorkSpace(containerName, volume string) {
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteVolumeMountPoint(containerName, volumeURLs)
		}
	}
	DeleteMountPoint(containerName)
	DeleteLayer(containerName)
}
