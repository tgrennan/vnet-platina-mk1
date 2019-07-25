// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build ignore

package main

import (
	"github.com/platinasystems/vnet"
	"github.com/platinasystems/xeth"
)

type spList []uint

func subportsMatchingPort(targetport uint) (numsubports uint, subportlist spList) {
	subportlist = []uint{0xf, 0xf, 0xf, 0xf}
	vnet.Ports.Foreach(func(ifname string, pe *vnet.PortEntry) {
		if pe.Devtype == xeth.XETH_DEVTYPE_XETH_PORT &&
			pe.Portindex == int16(targetport) {
			subportlist[numsubports] = uint(pe.Subportindex)
			numsubports++
		}
	})
	return
}

func (subportlist spList) contains(targetsubport uint) bool {
	for _, subport := range subportlist {
		if subport == targetsubport {
			return true
		}
	}
	return false
}
