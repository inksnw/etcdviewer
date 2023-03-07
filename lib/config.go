package lib

import (
	"gopkg.in/yaml.v2"
	"os"
)

func Check(err error) {
	if err != nil {
		panic(err)
	}
}

type Config struct {
	Host string `yaml:"host"`
	CA   string `yaml:"ca"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

func InitConfig() {
	yamlFile, err := os.ReadFile("./config.yaml")
	Check(err)
	err = yaml.Unmarshal(yamlFile, &config)
	Check(err)
}
