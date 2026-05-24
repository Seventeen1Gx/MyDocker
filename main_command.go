package main

import (
	"fmt"
	"my-docker/container"
	"my-docker/network"
	"my-docker/subsystem"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "Create a container with namespace and cgroups limit",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
		cli.StringFlag{
			Name:  "v",
			Usage: "volume, format: hostPath:containerPath",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment variables",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network",
		},
		cli.StringSliceFlag{
			Name:  "p",
			Usage: "port mapping",
		},
	},
	Action: func(ctx *cli.Context) error {
		if ctx.NArg() < 2 {
			return fmt.Errorf("Missing container command")
		}

		tty := ctx.Bool("ti")
		detach := ctx.Bool("d")
		if tty && detach {
			return fmt.Errorf("ti and d param can not both provided")
		}

		m := ctx.String("m")
		cpushare := ctx.String("cpushare")
		cpuset := ctx.String("cpuset")
		var resConf *subsystem.ResourceConfig
		if m != "" || cpushare != "" || cpuset != "" {
			resConf = &subsystem.ResourceConfig{}
			resConf.MemoryMax = m
			resConf.CpuSet = cpuset
			resConf.CpuWeight = cpushare
		}

		volume := ctx.String("v")
		containerName := ctx.String("name")

		envArray := ctx.StringSlice("e")

		network := ctx.String("net")
		portmapping := ctx.StringSlice("p")

		imageName := ctx.Args().First()
		cmdArray := ctx.Args().Tail()

		Run(tty, cmdArray, resConf, imageName, containerName, volume, envArray, network, portmapping)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call is outside",
	Action: func(ctx *cli.Context) error {
		log.Info("init come on")
		err := container.RunContainerInitProcess()
		return err
	},
}

var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "Commit a container into an image",
	Action: func(ctx *cli.Context) error {
		if ctx.NArg() < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := ctx.Args().Get(0)
		imageName := ctx.Args().Get(1)
		commitContainer(imageName, containerName)
		return nil
	},
}

var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the containers",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Please input your container name")
		}
		containerName := context.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(context *cli.Context) error {
		// 第二次执行：Cgo 代码已执行，直接返回
		if os.Getenv(ENV_EXEC_PID) != "" {
			return nil
		}

		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing container name or command")
		}

		containerName := context.Args().Get(0)
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}

		ExecContainer(containerName, commandArray)
		return nil
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a running container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container name")
		}
		containerName := context.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove a stopped container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container name")
		}
		containerName := context.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}

var networkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{
		{
			Name:  "create",
			Usage: "create a container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()
				err := network.CreateNetwork(context.String("driver"), context.String("subnet"), context.Args().Get(0))
				if err != nil {
					log.Errorf("Create network error: %v", err)
				}
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				network.Init()
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()
				err := network.DeleteNetwork(context.Args().Get(0))
				if err != nil {
					log.Errorf("Delete network error: %v", err)
				}
				return nil
			},
		},
	},
}
