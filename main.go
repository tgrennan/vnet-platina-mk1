// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// This is the XETH control daemon for Platina's Mk1 TOR switch.
// Build it with,
//	go build
//	zip drivers vnet-platina-mk1
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/platinasystems/fe1"
	fe1a "github.com/platinasystems/firmware-fe1a"
	"github.com/platinasystems/redis"
	vnetfe1 "github.com/platinasystems/vnet/devices/ethernet/switch/fe1"
	yaml "gopkg.in/yaml.v2"
)

func main() {
	const usage = "vnetd [install, version, license, patents]"
	var err error
	f := mk1Main
	stub := func() error { return nil }

	redis.DefaultHash = "platina-mk1"
	vnetfe1.AddPlatform = fe1.AddPlatform
	vnetfe1.Init = fe1.Init

	for _, arg := range os.Args[1:] {
		switch strings.TrimLeft(arg, "-") {
		case "install":
			f = stub
			err = install()
		case "version":
			f = stub
			fmt.Println(Version)
		case "copyright", "license":
			f = stub
			err = marshalOut(licenses())
		case "patents":
			f = stub
			err = marshalOut(patents())
		case "h", "help", "usage":
			fmt.Println(usage)
			return
		default:
			err = fmt.Errorf("%q unknown", arg)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err = f(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func marshalOut(m map[string]string) error {
	b, err := yaml.Marshal(m)
	if err == nil {
		os.Stdout.Write(b)
	}
	return err
}

func licenses() map[string]string {
	return map[string]string{
		"fe1":  fe1.License,
		"fe1a": fe1a.License,

		"vnet-platina-mk1": License,
	}
}

func patents() map[string]string {
	return map[string]string{
		"fe1": fe1.Patents,
	}
}
