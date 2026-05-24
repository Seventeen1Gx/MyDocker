package network

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type BridgeNetworkDriver struct{}

func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (d *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Create ParseCIDR error: %v", err)
		return nil, err
	}
	ones, _ := ipNet.Mask.Size()

	// 通过 IPAM 分配网关 IP
	// 通常就是网段的第一个可用 IP 地址
	// 192.169.0.0/24 => 192.169.0.1
	gatewayIP, err := ipAllocator.Allocate(ipNet)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Create Allocate error: %v", err)
		return nil, err
	}

	// 设备名
	devName := d.DevName(name)
	dev := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: devName,
		},
	}

	// 创建 Bridge 虚拟设备
	// ip link add {devName} type bridge
	if err := netlink.LinkAdd(dev); err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return nil, fmt.Errorf("bridge device %s already exists", devName)
		}
		log.Errorf("BridgeNetworkDriver Create LinkAdd error: %v", err)
		return nil, err
	}

	// 设置 Bridge 虚拟设备 IP 地址
	// ip addr add {ipNet} dev {devName}
	// 这里的 ipNet 必须是网关地址!!! => 192.169.0.1/24
	addr, err := netlink.ParseAddr(fmt.Sprintf("%s/%d", gatewayIP.String(), ones))
	if err != nil {
		log.Errorf("BridgeNetworkDriver Create ParseAddr error: %v", err)
		return nil, err
	}
	if err := netlink.AddrAdd(dev, addr); err != nil {
		log.Errorf("BridgeNetworkDriver Create AddrAdd error: %v", err)
		return nil, err
	}

	// 启动 Bridge 虚拟设备
	// ip link set {devName} up
	if err := netlink.LinkSetUp(dev); err != nil {
		log.Errorf("BridgeNetworkDriver Create LinkSetUp error: %v", err)
		return nil, err
	}

	// 设置 MASQUERADE/SNAT 规则
	// iptables -t nat -A POSTROUTING -s <ips> ! -o <bridgeName> -j MASQUERADE
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE",
		subnet, devName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)

	// 执行 iptables 命令配置规则
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
		return nil, fmt.Errorf("iptables cmt error: %s", err)
	}

	return &Network{Name: name, IpRange: ipNet, IP: gatewayIP, Driver: d.Name()}, nil
}

func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	devName := d.DevName(network.Name)
	dev, err := netlink.LinkByName(devName)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Connect LinkByName error: %v", err)
		return err
	}

	vethName := fmt.Sprintf("veth-%s", endpoint.ID[:5])
	peerName := fmt.Sprintf("vethp-%s", endpoint.ID[:5])
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:        vethName,
			MasterIndex: dev.Attrs().Index, // 等价于 ip link set master
		},
		PeerName: peerName,
	}

	// 创建 Veth 虚拟设备
	// ip link add {vethName} type veth peer name {peerName}
	if err := netlink.LinkAdd(veth); err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("veth device %s already exists", vethName)
		}
		log.Errorf("BridgeNetworkDriver Connect LinkAdd error: %v", err)
		return err
	}

	// 启动 Veth 设备
	// ip link set {vethName} up
	if err := netlink.LinkSetUp(veth); err != nil {
		log.Errorf("BridgeNetworkDriver Connect LinkSetUp error: %v", err)
		netlink.LinkDel(veth)
		return err
	}

	endpoint.Device = *veth
	return nil
}

func (d *BridgeNetworkDriver) Delete(network Network) error {
	devName := d.DevName(network.Name)
	dev, err := netlink.LinkByName(devName)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Delete LinkByName error: %v", err)
		return err
	}
	// ip link del <devName>
	err = netlink.LinkDel(dev)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Delete LinkDel error: %v", err)
		return err
	}
	// 清理 MASQUERADE 规则
	iptablesCmd := fmt.Sprintf(
		"-t nat -D POSTROUTING -s %s ! -o %s -j MASQUERADE",
		network.IpRange.String(),
		devName,
	)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	if err := cmd.Run(); err != nil {
		log.Warnf("delete masquerade rule failed: %v", err)
	}
	return nil
}

func (d *BridgeNetworkDriver) Disconnect(network Network, endpoint *Endpoint) error {
	vethName := fmt.Sprintf("veth-%s", endpoint.ID[:5])
	link, err := netlink.LinkByName(vethName)
	if err != nil {
		log.Errorf("BridgeNetworkDriver Disconnect LinkByName error: %v", err)
		return err
	}
	return netlink.LinkDel(link)
}

func (d *BridgeNetworkDriver) DevName(name string) string {
	return fmt.Sprintf("br-%s", name)
}
