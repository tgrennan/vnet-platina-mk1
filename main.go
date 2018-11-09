// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// This is a goes plugin containing Platina's Mk1 TOR driver daemon.
// Build it with,
//	go build -buildmode=plugin
//	zip plugins vnet-platina-mk1.so
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
	redis.DefaultHash = "platina-mk1"
	if err := Main(os.Args[1:]...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Main(args ...string) error {
	for i, arg := range args {
		switch strings.TrimLeft(arg, "-") {
		case "version":
			return marshalOut(Versions())
		case "copyright", "license":
			return marshalOut(Licenses())
		case "patents":
			return marshalOut(Patents())
			return nil
		case "h", "help":
			fmt.Println("vnetd [version, license, patents]")
			return nil
		default:
			return fmt.Errorf("%q unknown", args[i])
		}
	}

	vnetfe1.AddPlatform = fe1.AddPlatform
	vnetfe1.Init = fe1.Init

	return mk1Main()
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
