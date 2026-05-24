package network

import (
	"encoding/json"
	"net"
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var (
	defaultNetworkPath = "/home/gxx/my-docker-test/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Network struct {
	Name    string     // 网络名
	IpRange *net.IPNet // 网段 192.169.0.0/24
	IP      net.IP     // 网关地址 192.169.0.1
	Driver  string     // 驱动名
}

type Endpoint struct {
	ID string `json:"id"`
	// 一对 Veth 设备
	Device netlink.Veth `json:"dev"`
	// 一端容器
	IPAddress net.IP `json:"ip"`
	// 一端网络
	Network *Network
	// 宿主机与容器端口映射关系
	PortMapping []string `json:"portmapping"`
}

type NetworkDriver interface {
	// 驱动名
	Name() string
	// 创建网络
	Create(subnet string, name string) (*Network, error)
	// 删除网络
	Delete(network Network) error
	// 将容器端点连接到网络
	Connect(network *Network, endpoint *Endpoint) error
	// 从网络断开容器端点
	Disconnect(network Network, endpoint *Endpoint) error
}

// 网络配置持久化
func (nw *Network) dump(dumpPath string) error {
	// 如果目录不存在，则创建目录
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}

	// 打开文件：不存在则创建，存在则清空
	nwPath := path.Join(dumpPath, nw.Name)
	nwFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer nwFile.Close()

	// 序列化网络配置为 JSON 格式并写入文件
	encoder := json.NewEncoder(nwFile)
	return encoder.Encode(nw)
}

func (nw *Network) load(dumpPath string) error {
	nwPath := path.Join(dumpPath, nw.Name)
	f, err := os.Open(nwPath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	return decoder.Decode(nw)
}

func (nw *Network) remove(dumpPath string) error {
	nwPath := path.Join(dumpPath, nw.Name)
	if _, err := os.Stat(nwPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		return os.RemoveAll(nwPath)
	}
}

func Init() error {
	var err error
	ipAllocator, err = NewIPAM()
	if err != nil {
		log.Errorf("Error Init IPAM: %v", err)
		return err
	}

	drivers = make(map[string]NetworkDriver)
	bridgeDriver := &BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = bridgeDriver

	networks = make(map[string]*Network)
	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
			return nil
		} else {
			log.Errorf("Error Init Network: %v", err)
			return err
		}
	}

	// 遍历配置目录，加载已有的网络配置
	err = filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		nwPathPre, nwName := path.Split(nwPath)
		nw := &Network{Name: nwName}
		if err := nw.load(nwPathPre); err != nil {
			log.Errorf("load network %s failed: %v", nwName, err)
			return err
		}
		networks[nwName] = nw
		return nil
	})
	return err
}
