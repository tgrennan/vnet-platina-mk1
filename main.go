// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// This is the XETH control daemon for Platina's Mk1 TOR switch.
// Build it with,
//	go build
//	zip drivers vnet-platina-mk1
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/platinasystems/buildid"
	"github.com/platinasystems/buildinfo"
	"github.com/platinasystems/fe1"
	fe1a "github.com/platinasystems/firmware-fe1a"
	"github.com/platinasystems/redis"
	vnetfe1 "github.com/platinasystems/vnet/devices/ethernet/switch/fe1"
	yaml "gopkg.in/yaml.v2"
)

const usage = `
usage:	vnet-platina-mk1
	vnet-platina-mk1 install
	vnet-platina-mk1 [show] {version, buildid, buildinfo, license, patents}`

var ErrUsage = errors.New(usage[1:])

func main() {
	args := os.Args[1:]
	assert := func(err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if len(args) == 0 {
		redis.DefaultHash = "platina-mk1"
		vnetfe1.AddPlatform = fe1.AddPlatform
		vnetfe1.Init = fe1.Init
		assert(mk1Main())
		return
	}
	arg := strings.TrimLeft(args[0], "-")
	if arg == "install" {
		if len(args) > 1 {
			assert(fmt.Errorf("%s", usage[1:]))
		}
		assert(install())
		return
	}
	if arg == "show" {
		args = args[1:]
	}
	for _, arg := range args {
		switch strings.TrimLeft(arg, "-") {
		case "version":
			fmt.Println(buildinfo.New().Version())
		case "buildid":
			s, err := buildid.New("/proc/self/exe")
			assert(err)
			fmt.Println(s)
		case "buildinfo":
			fmt.Println(buildinfo.New())
		case "copyright", "license":
			assert(marshalOut(licenses()))
		case "patents":
			assert(marshalOut(patents()))
		case "h", "help", "usage":
			fmt.Println(usage[1:])
		default:
			assert(fmt.Errorf("%q unknown", arg))
		}
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
