// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
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
	var err error
	f := mk1Main
	stub := func() error { return nil }

	redis.DefaultHash = "platina-mk1"
	vnetfe1.AddPlatform = fe1.AddPlatform
	vnetfe1.Init = fe1.Init

	for _, arg := range os.Args[1:] {
		switch strings.TrimLeft(arg, "-") {
		case "version":
			f = stub
			err = marshalOut(Versions())
		case "copyright", "license":
			f = stub
			err = marshalOut(Licenses())
		case "patents":
			f = stub
			err = marshalOut(Patents())
		case "h", "help":
			fmt.Println("vnetd [version, license, patents]")
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

func Licenses() map[string]string {
	return map[string]string{
		"fe1":  fe1.License,
		"fe1a": fe1a.License,

		"vnet-platina-mk1": License,
	}
}
func Patents() map[string]string {
	return map[string]string{
		"fe1": fe1.Patents,
	}
}

func Versions() map[string]string {
	return map[string]string{
		"fe1":  fe1.Version,
		"fe1a": fe1a.Version,

		"vnet-platina-mk1": Version,
	}
}
