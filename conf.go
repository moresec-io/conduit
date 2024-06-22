/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package conduit

import (
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/moresec-io/conduit/pkg/log"

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

type Config struct {
	Server struct {
		Enable bool `yaml:"enable"`
		Proxy  struct {
			Mode   string `yaml:"mode"`
			Listen string `yaml:"listen"`
		}
		Cert struct {
			CertFile string `yaml:"cert_file"`
			KeyFile  string `yaml:"key_file"`
			CaFile   string `yaml:"ca_file"`
		} `yaml:"cert"`
	} `yaml:"server"`

	Client struct {
		Enable bool `yaml:"enable"`
		Proxy  struct {
			Mode      string     `yaml:"mode"`
			LocalMode string     `yaml:"local_mode"`
			Listen    string     `yaml:"listen"` // for transparent
			CheckTime int        `yaml:"check_time"`
			Transfers []struct { // dstA -> B -> dstC
				Dst   string `yaml:"dst"`    // :9092
				Proxy string `yaml:"proxy"`  // 192.168.111.149
				DstTo string `yaml:"dst_to"` // 127.0.0.1:9092
			} `yaml:"transfers"`
			Timeout    int `yaml:"timeout"`
			ServerPort int `yaml:"server_port"`
		} `yaml:"proxy"`
		Cert struct {
			CertFile string `yaml:"cert_file"`
			KeyFile  string `yaml:"key_file"`
		} `yaml:"cert"`
	} `yaml:"client"`

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
	data, err := ioutil.ReadFile(file)
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
