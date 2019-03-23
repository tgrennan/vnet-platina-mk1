// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/platinasystems/vnet"
	"github.com/platinasystems/vnet/ip"
	"github.com/platinasystems/vnet/ip4"
)

type unresolvedArper struct {
	vnet.Event
	mk1          *Mk1
	sequence     uint
	pollInterval float64 // in seconds
}

func (p *unresolvedArper) addEvent(dt float64) {
	p.mk1.vnet.SignalEventAfter(p, dt)
}

func (p *unresolvedArper) String() string {
	return fmt.Sprintf("unresolvedArper ping")
}

func (p *unresolvedArper) EventAction() {
	p.addEvent(p.pollInterval)
	im4 := ip4.GetMain(&p.mk1.vnet)
	im4.ForeachUnresolved(func(fi ip.FibIndex, p net.IPNet) {
		xargs := []string{"ping", "-q", "-c", "1", "-W", "1", p.IP.String()}
		netns := im4.FibNameForIndex(fi)
		if netns != "default" {
			xargs = append([]string{"ip", "netns", "exec", netns}, xargs...)
		}
		if true {
			go func() {
				exec.Command(xargs[0], xargs[1:]...).Run()
			}()
		}
	})
	p.sequence++
}
