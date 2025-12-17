/*
 * This file is part of open-snell.
 * open-snell is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 * open-snell is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 * You should have received a copy of the GNU General Public License
 * along with open-snell.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/golang/glog"
	"gopkg.in/ini.v1"

	"github.com/icpz/open-snell/components/snell"
	"github.com/icpz/open-snell/constants"
)

type Config struct {
	ListenAddr string
	ObfsType   string
	PSK        string
}

func parseConfig() (*Config, error) {
	var (
		configFile string
		listenAddr string
		obfsType   string
		psk        string
		version    bool
	)

	flag.StringVar(&configFile, "c", "", "configuration file path")
	flag.StringVar(&listenAddr, "l", "0.0.0.0:18888", "server listen address")
	flag.StringVar(&obfsType, "obfs", "", "obfs type")
	flag.StringVar(&psk, "k", "", "pre-shared key")
	flag.BoolVar(&version, "version", false, "show open-snell version")

	flag.Parse()
	flag.Set("logtostderr", "true")

	if version {
		fmt.Printf("Open-snell server, version: %s\n", constants.Version)
		os.Exit(0)
	}

	log.Infof("Open-snell server, version: %s\n", constants.Version)

	if configFile != "" {
		log.Infof("Configuration file specified, ignoring other flags\n")
		cfg, err := ini.Load(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s, %v", configFile, err)
		}
		sec, err := cfg.GetSection("snell-server")
		if err != nil {
			return nil, fmt.Errorf("section 'snell-server' not found in config file %s", configFile)
		}

		listenAddr = sec.Key("listen").String()
		obfsType = sec.Key("obfs").String()
		psk = sec.Key("psk").String()
	}

	if obfsType == "none" || obfsType == "off" {
		obfsType = ""
	}

	return &Config{
		ListenAddr: listenAddr,
		ObfsType:   obfsType,
		PSK:        psk,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v\n", err)
	}

	sn, err := snell.NewSnellServer(cfg.ListenAddr, cfg.PSK, cfg.ObfsType)
	if err != nil {
		log.Fatalf("Failed to initialize snell server %v\n", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	sn.Close()
}
