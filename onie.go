// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/platinasystems/redis"
)

func onie(fn string) (string, error) {
	// look for onie file in platform then i2c device dirs
	for _, dir := range []string{
		"/sys/devices/platform/platina-mk1/onie",
		"/sys/bus/i2c/devices/0-0051/onie",
	} {
		if _, err := os.Stat(dir); err == nil {
			b, err := ioutil.ReadFile(filepath.Join(dir, fn))
			return string(b), err
		}
	}
	// before we had an onie device driver we had a go-lang eeprom reader
	// that published fields to redis
	rfn, found := map[string]string{
		"device_version": "eeprom.DeviceVersion",
		"num_macs":       "eeprom.NEthernetAddress",
		"mac_base":       "eeprom.BaseEthernetAddress",
	}[fn]
	if !found {
		return "", fmt.Errorf("%s: invalid", fn)
	}
	return redis.Hget(redis.DefaultHash, rfn)
}
