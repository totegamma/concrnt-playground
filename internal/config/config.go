package config

import (
	"os"

	"github.com/go-yaml/yaml"

	"github.com/totegamma/concrnt-playground"
)

type Config struct {
	NodeInfo NodeInfo `yaml:"nodeInfo"`
	Server   Server   `yaml:"server"`
}

type NodeInfo struct {
	FQDN         string `yaml:"fqdn"`
	PrivateKey   string `yaml:"privatekey"`
	Registration string `yaml:"registration"` // open, invite, close
	SiteKey      string `yaml:"sitekey"`
	Layer        string `yaml:"layer"`

	// ---
	CSID string
}

type Server struct {
	PostgresDsn     string `yaml:"postgresDsn"`
	RedisAddr       string `yaml:"redisAddr"`
	RedisDB         int    `yaml:"redisDB"`
	MemcachedAddr   string `yaml:"memcachedAddr"`
	EnableTrace     bool   `yaml:"enableTrace"`
	TraceEndpoint   string `yaml:"traceEndpoint"`
	RepositoryPath  string `yaml:"repositoryPath"`
	CaptchaSitekey  string `yaml:"captchaSitekey"`
	CaptchaSecret   string `yaml:"captchaSecret"`
	VapidPublicKey  string `yaml:"vapidPublicKey"`
	VapidPrivateKey string `yaml:"vapidPrivateKey"`
}

func Load(path string) (Config, error) {

	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = yaml.NewDecoder(file).Decode(&config)
	if err != nil {
		return Config{}, err
	}

	csid, err := concrnt.PrivKeyToAddr(config.NodeInfo.PrivateKey, "ccs")
	if err != nil {
		panic(err)
	}

	config.NodeInfo.CSID = csid

	return config, nil
}
