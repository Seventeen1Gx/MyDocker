package network

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type IPAM struct {
	allocPath string            // 地址分配信息的持久化路径
	allocates map[string]string // 地址分配情况，网段 -> 位图
}

var defaultAllocatorPath = "/home/gxx/my-docker-test/ipam/subnet.json"

// 全局 IP 地址分配器
var ipAllocator *IPAM

func NewIPAM() (*IPAM, error) {
	ipam := &IPAM{
		allocPath: defaultAllocatorPath,
		allocates: make(map[string]string),
	}
	// 从文件中加载已有的 IP 分配记录
	if err := ipam.load(); err != nil {
		return nil, fmt.Errorf("load ipam records failed: %w", err)
	}
	return ipam, nil
}

// 从网段中分配一个可用 IP 地址
func (ipam *IPAM) Allocate(subnet *net.IPNet) (net.IP, error) {
	key := subnet.String()

	// 返回子网掩码的前缀长度和总位数
	// 例如对于网段 "127.0.0.0/8" 返回 8 和 32
	ones, bits := subnet.Mask.Size()

	// 如果之前没有分配过这个网段，则初始化网段的分配配置
	if _, exist := ipam.allocates[key]; !exist {
		// 网段内所有地址都没有分配，所以位图字符串全为 "0"
		bitMap := []byte(strings.Repeat("0", 1<<uint8(bits-ones)))
		// 网段的首个地址作为网络地址
		// 网段的末尾地址作为广播地址
		bitMap[0] = '1'
		bitMap[len(bitMap)-1] = '1'
		ipam.allocates[key] = string(bitMap)
	}

	// 遍历网段的位图数组
	for i := range ipam.allocates[key] {
		// 找到数组中为"0"的项和数组序号，即可以分配的 IP
		if ipam.allocates[key][i] == '0' {
			// 设置这个为 "0" 的序号值为 "1"，即分配这个IP
			// Go 的字符串创建之后就不能修改，所以通过转换成 byte 数组，修改后再转换成字符串赋值
			ipalloc := []byte((ipam.allocates)[key])
			ipalloc[i] = '1'
			ipam.allocates[key] = string(ipalloc)
			return ipByOffset(subnet.IP.To4(), i), ipam.dump() // 持久化变更
		}
	}

	return nil, fmt.Errorf("no available ip in subnet %s", subnet)
}

// 释放不再使用的 IP 地址
func (ipam *IPAM) Release(subnet *net.IPNet, ipAddr net.IP) error {
	offset := ipOffset(subnet.IP.To4(), ipAddr.To4())

	key := subnet.String()
	bm := ipam.allocates[key]
	if offset <= 0 || offset >= len(bm)-1 {
		return fmt.Errorf("invalid ip offset")
	}

	// 标记为未分配
	b := []byte(bm)
	b[offset] = '0'
	ipam.allocates[key] = string(b)

	return ipam.dump() // 持久化变更
}

// 返回从 subnetIP 开始第几个是 targetIP
func ipOffset(subnetIP, targetIP net.IP) int {
	s := binary.BigEndian.Uint32(subnetIP)
	t := binary.BigEndian.Uint32(targetIP)
	return int(t - s)
}

// 返回从 subnetIP 开始的第 offset 个 IP
func ipByOffset(subnetIP net.IP, offset int) net.IP {
	s := binary.BigEndian.Uint32(subnetIP)
	t := s + uint32(offset)
	return net.IPv4(byte(t>>24), byte(t>>16), byte(t>>8), byte(t))
}

func (ipam *IPAM) dump() error {
	dir := filepath.Dir(ipam.allocPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0644)
	}

	data, err := json.MarshalIndent(ipam.allocates, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ipam.allocPath, data, 0644)
}

func (ipam *IPAM) load() error {
	// 如果文件不存在，直接返回（初始状态）
	if _, err := os.Stat(ipam.allocPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	file, err := os.ReadFile(ipam.allocPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(file, &ipam.allocates)
}
