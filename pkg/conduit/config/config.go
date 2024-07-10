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

type Manager struct {
	Enable bool        `yaml:"enable"`
	Dial   config.Dial `yaml:"dial"`
}

type Server struct {
	Enable bool          `yaml:"enable"`
	Listen config.Listen `yaml:"listen"`
}

type Policy struct {
	Dst   string       `yaml:"dst"`              // :9092
	Proxy *config.Dial `yaml:"proxy,omitempty"`  // 192.168.111.149
	DstTo string       `yaml:"dst_to,omitempty"` // 127.0.0.1:9092
}

// TLS > Default TLS
type Client struct {
	Enable       bool     `yaml:"enable"`
	Listen       string   `yaml:"listen"` // for tcp transparent
	CheckTime    int      `yaml:"check_time"`
	Policies     []Policy `yaml:"policies"`
	DefaultProxy struct {
		Network    string     `yaml:"network"`
		ServerPort int        `yaml:"server_port"` // server addr is combined by dst:server_port
		TLS        config.TLS `yaml:"tls,omitempty"`
	} `yaml:"default_proxy"`
}

type Config struct {
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
	return err
}

func initCmd() error {
	flag.StringVar(&file, "f", defaultFile, "configuration file")
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
