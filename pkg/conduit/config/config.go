/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/config"

	"github.com/natefinch/lumberjack"
	"gopkg.in/yaml.v2"
)

var (
	Conf      *Config
	RotateLog *lumberjack.Logger

	h           bool
	file        string
	defaultFile string = "./conduit.yaml"
)

var (
	MarkIgnoreOurself = 1444
	MarkIpsetIP       = 1445
	MarkIpsetIPPort   = 1446
	MarkIpsetPort     = 1447
)

type Manager struct {
	Enable bool        `yaml:"enable"`
	Dial   config.Dial `yaml:"dial"`
}

type ForwardElem struct {
	Dst       string `yaml:"dst"` // :9092 or 192.168.0.2:9092
	PeerIndex int    `yaml:"peer_index"`
	DstAs     string `yaml:"dst_as"`
}

type Peer struct {
	Index     int        `yaml:"index"`
	Network   string     `yaml:"network"`
	Addresses []string   `yaml:"addresses"`
	TLS       config.TLS `yaml:"tls"`
}

// TLS > Default TLS
type Client struct {
	Enable       bool          `yaml:"enable"`
	Network      string        `yaml:"network"` // tcp, udp or tcp,udp
	Listen       string        `yaml:"listen"`  // for tcp transparent
	CheckTime    int           `yaml:"check_time"`
	ForwardTable []ForwardElem `yaml:"forward_table"`
	Peers        []Peer        `yaml:"peers"`
}

type Server struct {
	Enable        bool `yaml:"enable"`
	config.Listen `yaml:"listen"`
}

type Config struct {
	MachineID string `yaml:"-"`

	Manager Manager `yaml:"manager"`

	Server Server `yaml:"server"`

	Client Client `yaml:"client"`

	Log struct {
		Level    string `yaml:"level"`
		File     string `yaml:"file"`
		MaxSize  int    `yaml:"maxsize"`
		MaxRolls int    `yaml:"maxrolls"`
	} `yaml:"log"`
}

func Init() error {
	time.LoadLocation("Asia/Shanghai")

	err := initCmd()
	if err != nil {
		return err
	}

	err = initConf()
	if err != nil {
		return err
	}

	err = initLog()
	if err != nil {
		return err
	}
	id, err := machineid.ID()
	if err != nil {
		return err
	}
	Conf.MachineID = id
	return err
}

func initCmd() error {
	flag.StringVar(&file, "c", defaultFile, "configuration file")
	flag.BoolVar(&h, "h", false, "help")
	flag.Parse()
	if h {
		flag.Usage()
		return fmt.Errorf("invalid usage for command line")
	}
	return nil
}

func initConf() error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	Conf = &Config{}
	err = yaml.Unmarshal([]byte(data), Conf)
	return err
}

func initLog() error {
	level, err := log.ParseLevel(Conf.Log.Level)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	RotateLog = &lumberjack.Logger{
		Filename:   Conf.Log.File,
		MaxSize:    Conf.Log.MaxSize,
		MaxBackups: Conf.Log.MaxRolls,
	}
	log.SetOutput(RotateLog)
	return nil
}
