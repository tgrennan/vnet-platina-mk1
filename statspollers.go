// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/platinasystems/vnet"
	"github.com/platinasystems/xeth"
)

// One per each hw/sw interface from vnet.
type ifStatsPollerInterface struct {
	lastValues   map[string]uint64
	hfLastValues map[string]uint64
}

func (i *ifStatsPollerInterface) update(counter string, value uint64) (updated bool) {
	if i.lastValues == nil {
		i.lastValues = make(map[string]uint64)
	}
	if v, ok := i.lastValues[counter]; ok {
		if updated = v != value; updated {
			i.lastValues[counter] = value
		}
	} else {
		updated = true
		i.lastValues[counter] = value
	}
	return
}
func (i *ifStatsPollerInterface) updateHf(counter string, value uint64) (delta uint64, updated bool) {
	if i.hfLastValues == nil {
		i.hfLastValues = make(map[string]uint64)
	}
	portregex := regexp.MustCompile(`(packets|bytes) *`)
	if v, ok := i.hfLastValues[counter]; ok {
		if updated = v != value; updated {
			i.hfLastValues[counter] = value
			if portregex.MatchString(counter) {
				if value > v {
					delta = value - v
				}
			} else {
				delta = value
			}
		}
	} else {
		updated = true
		i.hfLastValues[counter] = value
	}
	return
}

//go:generate gentemplate -d Package=main -id ifStatsPollerInterface -d VecType=ifStatsPollerInterfaceVec -d Type=ifStatsPollerInterface github.com/platinasystems/elib/vec.tmpl

type ifStatsPoller struct {
	vnet.Event
	mk1          *Mk1
	sequence     uint
	hwInterfaces ifStatsPollerInterfaceVec
	swInterfaces ifStatsPollerInterfaceVec
	pollInterval float64 // pollInterval in seconds
	pubch        chan string
}

func (p *ifStatsPoller) publish(name, counter string, value uint64) {
	p.pubch <- fmt.Sprintf("%s.%s: %d", name, counter, value)
}

func (p *ifStatsPoller) addEvent(dt float64) {
	p.mk1.vnet.SignalEventAfter(p, dt)
}

func (p *ifStatsPoller) String() string {
	return fmt.Sprintf("redis stats poller sequence %d", p.sequence)
}

func (p *ifStatsPoller) EventAction() {
	// Schedule next event in 5 seconds; do before fetching counters so that time interval is accurate.
	p.addEvent(p.pollInterval)

	start := time.Now()
	s := fmt.Sprint("poll.start.time: ", start.Format(time.StampMilli))
	p.pubch <- s
	s = fmt.Sprint("poll.start.channel-length: ", len(p.pubch))
	p.pubch <- s

	p.mk1.pubHwIfConfig()

	// Publish all sw/hw interface counters even with zero values for first poll.
	// This was all possible counters have valid values in redis.
	// Otherwise only publish to redis when counter values change.
	includeZeroCounters := p.sequence == 0

	pubcount := func(ifname, counter string, value uint64) {
		counter = xCounter(counter)
		entry := xeth.Interface.Named(ifname)
		if value != 0 && entry != nil &&
			entry.DevType == xeth.XETH_DEVTYPE_XETH_PORT {
			if _, found := vnet.Ports[ifname]; found {
				xethif := xeth.Interface.Named(ifname)
				ifindex := xethif.Ifinfo.Index
				xeth.SetStat(ifindex, counter, value)
			}

		}
		p.publish(ifname, counter, value)
	}
	p.mk1.vnet.ForeachHwIfCounter(includeZeroCounters,
		p.mk1.unixInterfacesOnly,
		func(hi vnet.Hi, counter string, value uint64) {
			p.hwInterfaces.Validate(uint(hi))
			if p.hwInterfaces[hi].update(counter, value) && true {
				pubcount(hi.Name(&p.mk1.vnet), counter, value)
			}
		})

	p.mk1.vnet.ForeachSwIfCounter(includeZeroCounters,
		func(si vnet.Si, siName, counter string, value uint64) {
			p.swInterfaces.Validate(uint(si))
			if p.swInterfaces[si].update(counter, value) && true {
				pubcount(siName, counter, value)
			}
		})

	stop := time.Now()
	p.pubch <- fmt.Sprint("poll.stop.time: ", stop.Format(time.StampMilli))
	p.pubch <- fmt.Sprint("poll.stop.channel-length: ", len(p.pubch))

	p.mk1.vnet.ForeachHwIf(false, func(hi vnet.Hi) {
		h := p.mk1.vnet.HwIfer(hi)
		hw := p.mk1.vnet.HwIf(hi)
		// FIXME how to filter these in a better way?
		if strings.Contains(hw.Name(), "fe1-") ||
			strings.Contains(hw.Name(), "pg") ||
			strings.Contains(hw.Name(), "meth") {
			return
		}

		if hw.IsLinkUp() {
			sp := h.GetHwInterfaceFinalSpeed()
			// Send speed message to driver so ethtool can see it
			xethif := xeth.Interface.Named(hw.Name())
			ifindex := xethif.Ifinfo.Index
			xeth.Speed(int(ifindex), uint64(sp/1e6))
			if false {
				fmt.Println("FinalSpeed:", hw.Name(), ifindex, sp, uint64(sp/1e6))
			}
		}
	})

	p.sequence++
}

type fastIfStatsPoller struct {
	vnet.Event
	mk1          *Mk1
	sequence     uint
	hwInterfaces ifStatsPollerInterfaceVec
	swInterfaces ifStatsPollerInterfaceVec
	pollInterval float64 // pollInterval in milliseconds
	pubch        chan string
	msgCount     uint64
	hostname     string
}

func (p *fastIfStatsPoller) publish(data map[string]string) {
	for k, v := range data {
		p.pubch <- fmt.Sprintf("%s,%d,%s,%s", p.hostname, time.Now().UnixNano()/1000000, k, v)
	}
}

func (p *fastIfStatsPoller) addEvent(dt float64) {
	p.mk1.vnet.SignalEventAfter(p, dt)
}

func (p *fastIfStatsPoller) String() string {
	return fmt.Sprintf("redis stats poller sequence %d", p.sequence)
}

func (p *fastIfStatsPoller) EventAction() {
	// Schedule next event in 200 milliseconds; do before fetching counters so that time interval is accurate.
	//p.addEvent(p.pollInterval / 1000)

	// Publish all sw/hw interface counters even with zero values for first poll.
	// This was all possible counters have valid values in redis.
	// Otherwise only publish to redis when counter values change.
	var c = make(map[string]string)
	p.mk1.vnet.ForeachHighFreqHwIfCounter(true,
		p.mk1.unixInterfacesOnly,
		func(hi vnet.Hi, counter string, value uint64) {
			ifname := hi.Name(&p.mk1.vnet)
			p.hwInterfaces.Validate(uint(hi))
			delta, _ := p.hwInterfaces[hi].updateHf(counter, value)
			c[ifname] = c[ifname] + fmt.Sprint(delta) + ","
		})
	//if p.mk1.producer != nil{
	//	p.publish(c)
	//}
	p.sequence++
}
