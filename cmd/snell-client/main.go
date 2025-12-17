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
	ServerAddr string
	ObfsType   string
	ObfsHost   string
	PSK        string
	SnellVer   string
}

func parseConfig() (*Config, error) {
	var (
		configFile string
		listenAddr string
		serverAddr string
		obfsType   string
		obfsHost   string
		psk        string
		snellVer   string
		version    bool
	)

	flag.StringVar(&configFile, "c", "", "configuration file path")
	flag.StringVar(&listenAddr, "l", "0.0.0.0:18888", "client listen address")
	flag.StringVar(&serverAddr, "s", "", "snell server address")
	flag.StringVar(&obfsType, "obfs", "", "obfs type")
	flag.StringVar(&obfsHost, "obfs-host", "bing.com", "obfs host")
	flag.StringVar(&psk, "k", "", "pre-shared key")
	flag.BoolVar(&version, "version", false, "show open-snell version")

	flag.Parse()
	flag.Set("logtostderr", "true")

	if version {
		fmt.Printf("Open-snell client, version: %s\n", constants.Version)
		os.Exit(0)
	}

	log.Infof("Open-snell client, version: %s\n", constants.Version)

	if configFile != "" {
		log.Infof("Configuration file specified, ignoring other flags\n")
		cfg, err := ini.Load(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s, %v", configFile, err)
		}
		sec, err := cfg.GetSection("snell-client")
		if err != nil {
			return nil, fmt.Errorf("section 'snell-client' not found in config file %s", configFile)
		}

		listenAddr = sec.Key("listen").String()
		serverAddr = sec.Key("server").String()
		obfsType = sec.Key("obfs").String()
		obfsHost = sec.Key("obfs-host").String()
		psk = sec.Key("psk").String()
		snellVer = sec.Key("version").String()
	}

	if serverAddr == "" {
		return nil, fmt.Errorf("invalid empty server address")
	}

	if obfsHost == "" {
		log.Infof("Note: obfs host empty, using default bing.com\n")
		obfsHost = "bing.com"
	}

	if obfsType == "none" || obfsType == "off" {
		obfsType = ""
	}

	if snellVer == "" {
		snellVer = "2"
	}

	return &Config{
		ListenAddr: listenAddr,
		ServerAddr: serverAddr,
		ObfsType:   obfsType,
		ObfsHost:   obfsHost,
		PSK:        psk,
		SnellVer:   snellVer,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v\n", err)
	}

	sn, err := snell.NewSnellClient(
		cfg.ListenAddr,
		cfg.ServerAddr,
		cfg.ObfsType,
		cfg.ObfsHost,
		cfg.PSK,
		cfg.SnellVer == "2",
	)
	if err != nil {
		log.Fatalf("Failed to initialize snell client %v\n", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	sn.Close()
}
