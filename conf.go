package conduit

import (
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jumboframes/conduit/pkg/log"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"gopkg.in/yaml.v2"
)

var (
	Conf      *Config
	RotateLog *rotatelogs.RotateLogs

	h           bool
	file        string
	defaultFile string = "./github.com/jumboframes/conduit.yaml"
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
			Mode       string   `yaml:"mode"`
			Listen     string   `yaml:"listen"`
			Chain      string   `yaml:"chain"`
			Addrs      []string `yaml:"addrs"`
			Timeout    int      `yaml:"timeout"`
			ServerPort int      `yaml:"server_port"`
		} `yaml:"proxy"`
		Cert struct {
			CertFile string `yaml:"cert_file"`
			KeyFile  string `yaml:"key_file"`
		} `yaml:"cert"`
	} `yaml:"client"`

	Log struct {
		Level    string `yaml:"level"`
		File     string `yaml:"file"`
		MaxSize  int64  `yaml:"maxsize"`
		MaxRolls uint   `yaml:"maxrolls"`
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
	RotateLog, err = rotatelogs.New(Conf.Log.File,
		rotatelogs.WithRotationCount(Conf.Log.MaxRolls),
		rotatelogs.WithRotationSize(Conf.Log.MaxSize))
	if err != nil {
		return err
	}
	log.SetOutput(RotateLog)
	return nil
}
