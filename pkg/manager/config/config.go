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
	defaultFile string = "./conduit_manager.yaml"
)

type DB struct {
	Driver      string      `yaml:"driver"` // support mysql or sqlite
	Address     string      `yaml:"address"`
	DB          string      `yaml:"db"`
	Options     string      `yaml:"options"`
	Debug       bool        `yaml:"debug"`
	Username    string      `yaml:"username"`      // mysql only
	Password    string      `yaml:"password"`      // mysql only
	MaxIdleConn int64       `yaml:"max_idle_conn"` // mysql only
	MaxOpenConn int64       `yaml:"max_open_conn"` // mysql only
	TLS         *config.TLS `yaml:"tls"`           // mysql only
}

type Cert struct {
	CA struct {
		NotAfter     string `yaml:"not_after"` // 0,1,0 means 0 year 1 month 0 day
		CommonName   string `yaml:"common_name"`
		Organization string `yaml:"organization"`
	}
	Cert struct {
		NotAfter     string `yaml:"not_after"`
		CommonName   string `yaml:"common_name"`
		Organization string `yaml:"organization"`
	}
}

type ControlPlane struct {
	Listen config.Listen `yaml:"listen"`
}

type ConduitManager struct {
	Listen config.Listen `yaml:"listen"`
}

type Config struct {
	ControlPlane ControlPlane `yaml:"control_plane"`

	ConduitManager ConduitManager `yaml:"conduit_manager"`

	DB DB `yaml:"db"`

	Cert Cert `yaml:"cert"`

	Log struct {
		Level    string `yaml:"level"`
		File     string `yaml:"file"`
		MaxSize  int    `yaml:"maxsize"`
		MaxRolls int    `yaml:"maxrolls"`
	} `yaml:"log"`
}

func NewConfig() (*Config, error) {
	time.LoadLocation("Asia/Shanghai")
	err := initCmd()
	if err != nil {
		log.Warnf("new config, init cmd err: %s", err)
		return nil, err
	}

	err = initConf()
	if err != nil {
		return nil, err
	}

	err = initLog()
	if err != nil {
		return nil, err
	}
	log.Infof(`
==================================================
                MANAGER STARTS
==================================================`)

	return Conf, err
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
