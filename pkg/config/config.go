package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Command struct {
	Name      string            `yaml:"name"`
	Command   []string          `yaml:"command"`
	Env       map[string]string `yaml:"env"`
	Exclusion bool              `yaml:"exclusion"`
	Timeout   int               `yaml:"timeout"` // second
}

type Config struct {
	Port     int       `yaml:"port"`
	Commands []Command `yaml:"commands"`
}

func Load(f string) (Config, error) {
	var conf Config

	file, err := os.Open(f)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(&conf); err != nil {
		return Config{}, err
	}

	for _, v := range conf.Commands {
		if len(v.Command) == 0 {
			return Config{}, fmt.Errorf("config: parse error at %s", v.Name)
		}
	}

	return conf, nil
}
