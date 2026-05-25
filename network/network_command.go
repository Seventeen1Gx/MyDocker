package network

import (
	"fmt"
	"my-docker/container"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func CreateNetwork(driver, subnet, name string) error {
	// 调用指定驱动的 Create 方法创建网络
	nw, err := drivers[driver].Create(subnet, name)
	if err != nil {
		return fmt.Errorf("driver create network failed: %w", err)
	}

	// 将网络配置持久化到文件
	return nw.dump(defaultNetworkPath)
}

func Connect(networkName string, cinfo *container.ContainerInfo) error {
	// 获取网络信息
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	// 通过 IPAM 分配容器 IP
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}
	cinfo.IP = ip.String()

	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.ID, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}

	// 调用驱动的 Connect 方法配置网络，补上端点的 Veth 信息
	if err := drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}

	// 进入容器 Namespace 配置 IP 和路由
	if err := configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	// 配置端口映射
	return configPortMapping(ep)
}

// 容器退出时调用：断开网络 + 清理资源
func Disconnect(networkName string, cinfo *container.ContainerInfo) error {
	if networkName == "" {
		return nil
	}
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.ID, networkName),
		IPAddress:   net.ParseIP(cinfo.IP),
		Network:     nw,
		PortMapping: cinfo.PortMapping,
	}

	if err := ipAllocator.Release(nw.IpRange, ep.IPAddress); err != nil {
		return fmt.Errorf("Release Ip Addr Fail: %v", err)
	}

	if err := drivers[nw.Driver].Disconnect(*nw, ep); err != nil {
		return fmt.Errorf("Disconnect Fail: %s", err)
	}

	if err := cleanPortMapping(ep); err != nil {
		return fmt.Errorf("Clean Port Mapping Fail: %v", err)
	}

	return nil
}

// 配置容器网络端点的地址和路由
func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	// 通过网络端点中 "Veth" 的另一端
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	// 将容器的网络端点加入到容器的网络空间中
	// 并使这个函数下面的操作都在这个网络空间中进行
	// 执行完函数后，恢复为默认的网络空间
	defer enterContainerNetns(&peerLink, cinfo)()

	// 走到这里已经处在容器的 Net Namespace 中

	ones, _ := ep.Network.IpRange.Mask.Size()

	// 设置 Peer 虚拟设备的 IP 地址
	// 网段 192.168.1.0/24 容器 192.168.1.2 => 192.168.1.2/24
	addr, err := netlink.ParseAddr(fmt.Sprintf("%s/%d", ep.IPAddress.String(), ones))
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(peerLink, addr)
	if err != nil {
		return err
	}

	// 启动 Peer 虚拟设备
	err = netlink.LinkSetUp(peerLink)
	if err != nil {
		return err
	}

	// 启动容器内的 lo 虚拟设备
	loLink, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("lo LinkByName err: %v", err)
	}
	err = netlink.LinkSetUp(loLink)
	if err != nil {
		return err
	}

	// 添加路由 route add
	if err = netlink.RouteAdd(&netlink.Route{
		// 从 peer 网卡出去
		LinkIndex: peerLink.Attrs().Index,
		// 下一条给谁
		Gw: ep.Network.IP,
		// 作用范围：所有 IP 地址
		Scope: netlink.SCOPE_UNIVERSE,
	}); err != nil {
		return err
	}

	return nil
}

// 将容器的网络端点加入到容器的网络空间中
// 并锁定当前程序所执行的线程，使当前线程进入到容器的网络空间
// 返回值是一个函数指针，执行这个返回函数才会退出容器的网络空间，回归到宿主机的网络空间
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	// 找到容器的 Net Namespace 在 /proc/[pid]/ns/net 中
	// 打开这个文件的文件描述符就可以来操作 Net Namespace
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.PID), os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("error get container net namespace, %v", err)
	}

	// 取到文件的文件描述符
	nsFD := f.Fd()

	// 锁定当前程序所执行的线程，如果不锁定操作系统线程的话
	// Go 语言的 goroutine 可能会被调度到别的线程上去
	// 就不能保证一直在所需要的网络空间中了
	// 所以调用 runtime.LockOSThread 锁定当前程序执行的线程
	runtime.LockOSThread()

	// 修改网络端点 Veth 的另外一端，将其移动到容器的 Net Namespace 中
	// ip link set <设备名> netns <目标网络命名空间>
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns , %v", err)
	}

	// 通过 netns.Get 方法获得当前网络的 Net Namespace
	// 以便后面从容器的 Net Namespace 中退出，回到原本网络的 Net Namespace 中
	origins, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// 调用 netns.Set 方法，将当前进程加入容器的 Net Namespace
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("error set netns, %v", err)
	}

	// 返回之前 Net Namespace 的函数
	// 在容器的网络空间中，执行完容器配置之后调用此函数就可以将程序恢复到原生的 Net Namespace
	return func() {
		// 恢复到上面获取到的之前的 Net Namespace
		netns.Set(origins)
		// 关闭 Namespace 文件
		origins.Close()
		// 取消对当前程序的线程锁定
		runtime.UnlockOSThread()
		// 关闭 Namespace 文件
		f.Close()
	}
}

// 配置端口映射
func configPortMapping(ep *Endpoint) error {
	// 遍历容器端口映射列表
	for _, pm := range ep.PortMapping {
		// 分割成宿主机的端口和容器的端口
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			log.Errorf("port mapping format error, %v", pm)
			continue
		}

		// 将宿主机的端口请求转发到容器的地址和端口上
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)

		// 执行 iptables 命令，添加端口映射转发规则
		output, err := cmd.Output()
		if err != nil {
			log.Errorf("iptables Output, %v", output)
			continue
		}

		// PREROUTING 针对宿主机外部访问容器的流量
		// OUTPUT 针对宿主机本地访问容器的流量
		iptablesCmd = fmt.Sprintf("-t nat -A OUTPUT -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		output, err = cmd.Output()
		if err != nil {
			log.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return nil
}

// 清理端口映射：把之前加的规则删掉
func cleanPortMapping(ep *Endpoint) error {
	for _, pm := range ep.PortMapping {
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			continue
		}

		// 注意：这里是 -D DELETE，不是 -A ADD
		iptablesCmd := fmt.Sprintf(
			"-t nat -D PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1],
		)

		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		_ = cmd.Run() // 忽略错误，不存在也没关系

		iptablesCmd = fmt.Sprintf(
			"-t nat -D OUTPUT -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1],
		)

		cmd = exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		_ = cmd.Run() // 忽略错误，不存在也没关系
	}
	return nil
}

func ListNetwork() {
	// 参数分别为：输出流、最小列宽、列间距、填充字符、对齐方式
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// 输出表头
	fmt.Fprint(w, "NAME\tIpRange\tIP\tDriver\n")

	// 遍历全局 networks 字典，输出每个网络的详细信息
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			nw.Name,
			nw.IpRange.String(),
			nw.IP,
			nw.Driver,
		)
	}

	// 将缓冲区的所有内容刷新到标准输出
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

func DeleteNetwork(networkName string) error {
	// 查找网络是否存在于全局 networks 字典中
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	// 释放网络网关的 IP
	if err := ipAllocator.Release(nw.IpRange, nw.IP); err != nil {
		return fmt.Errorf("Error Remove Network gateway ip: %s", err)
	}

	// 调用驱动的 Delete 方法删除网络
	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("Error Remove Network DriverError: %s", err)
	}

	// 从网络的配置目录中删除该网络对应的配置文件
	return nw.remove(defaultNetworkPath)
}
