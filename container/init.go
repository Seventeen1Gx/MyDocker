package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func RunContainerInitProcess() error {
	cmdArray := readUserCommand()
	if len(cmdArray) == 0 {
		return fmt.Errorf("Run container get user command error, cmdArray is nil")
	}

	setUpMount()

	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error: %v", err)
		return err
	}
	log.Infof("Find path %s", path)
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf("exec command err: %v", err)
	}

	return nil
}

func readUserCommand() []string {
	// 从管道读取父进程发送过来的用户命令
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}

func setUpMount() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("get current location error: %v", err)
		return
	}
	log.Infof("current location is %s", pwd)

	// 防止容器内挂载影响到外部宿主
	if err := syscall.Mount("none", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		log.Errorf("mount none to / error: %v", err)
		return
	}

	pivotRoot(pwd)

	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		log.Errorf("mount proc error: %v", err)
		return
	}
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		log.Errorf("mount tmpfs error: %v", err)
		return
	}
}

func pivotRoot(root string) error {
	/*
	   因为 pivot_root 有硬性规定：新根目录必须是一个「挂载点」，不能是普通目录。
	   你准备用 busybox 作为新根 / 但 busybox 只是一个普通文件夹不是挂载点 → pivot_root 直接拒绝工作
	   所以自己挂载自己
	*/

	// syscall.MS_BIND 绑定挂载，即目录映射
	// syscall.MS_REC 递归挂载，子目录也挂载
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs to itself error: %v", err)
	}

	// 因为 pivot_root 需要一个临时目录来存放旧的根文件系统
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.MkdirAll(pivotDir, 0777); err != nil {
		return fmt.Errorf("mkdir pivot_root dir error: %v", err)
	}

	// 当前进程的根目录挂载到 root 上，旧的根目录挂载到 pivotDir 上
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v error: %v", root, err)
	}

	// 修改当前工作目录到根目录
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / error: %v", err)
	}

	// 卸载旧的根文件系统
	pivotDir = filepath.Join("/", ".pivot_root")
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir error: %v", err)
	}
	return os.Remove(pivotDir)
}
