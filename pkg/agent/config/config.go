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

type CertKey struct {
	Cert string `yaml:"cert" json:"cert"`
	Key  string `yaml:"key" json:"key"`
}

type TLS struct {
	Enable             bool      `yaml:"enable" json:"enable"`
	MTLS               bool      `yaml:"mtls" json:"mtls"`
	CACerts            []string  `yaml:"ca_certs" json:"ca_certs"`                         // ca certs paths
	Certs              []CertKey `yaml:"certs" json:"certs"`                               // certs paths
	InsecureSkipVerify bool      `yaml:"insecure_skip_verify" json:"insecure_skip_verify"` // for client use
}

type Listen struct {
	Network        string `yaml:"network" json:"network"`
	Addr           string `yaml:"addr" json:"addr"`
	AdvertisedAddr string `yaml:"advertised_addr,omitempty" json:"advertised_addr"`
	TLS            TLS    `yaml:"tls,omitempty" json:"tls"`
}

type Dial struct {
	Network        string   `yaml:"network" json:"network"`
	Addrs          []string `yaml:"addrs" json:"addrs"`
	AdvertisedAddr string   `yaml:"advertised_addr,omitempty" json:"advertised_addr"`
	TLS            TLS      `yaml:"tls,omitempty" json:"tls"`
}

type Config struct {
	Conduit struct {
		Enable bool `yaml:"enable"`
		Dial   Dial `yaml:"dial"`
	} `yaml:"conduit"`

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
			Listen    string     `yaml:"listen"` // for transparent
			CheckTime int        `yaml:"check_time"`
			Transfers []struct { // dstA -> B -> dstC
				Dst   string `yaml:"dst"`    // :9092
				Proxy string `yaml:"proxy"`  // 192.168.111.149
				DstTo string `yaml:"dst_to"` // 127.0.0.1:9092
				TLS   TLS    `yaml:"tls"`
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
