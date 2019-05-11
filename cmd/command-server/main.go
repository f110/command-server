package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/f110/command-server/pkg/config"
	"github.com/f110/command-server/pkg/server"
	"github.com/spf13/pflag"
)

func commandServer(confFile string) error {
	conf, err := config.Load(confFile)
	if err != nil {
		return err
	}
	s := server.NewCommandServer(conf.Commands)
	log.Printf("Start listen :%d", conf.Port)
	return http.ListenAndServe(fmt.Sprintf(":%d", conf.Port), s)
}

func main() {
	confFile := "config.yaml"
	fs := pflag.NewFlagSet("command-server", pflag.ExitOnError)
	fs.StringVar(&confFile, "conf", confFile, "Config file")
	if err := fs.Parse(os.Args); err != nil {
		os.Exit(1)
	}

	if err := commandServer(confFile); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}
