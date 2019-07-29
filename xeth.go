// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/platinasystems/fe1/fe1xeth"
	"github.com/platinasystems/vnet"
	"github.com/platinasystems/xeth"
)

const (
	NPorts        = 32
	NSubPorts     = 4
	TopPortVid    = 3999
	BottomPortVid = TopPortVid - (NPorts * NSubPorts) + 1
	XidPortMask   = ^xeth.Xid(xeth.VlanVidMask)
)

const (
	CopperBit = iota
	Fec74Bit
	Fec91Bit
	KrBit
)

var mk1XethBreaks, mk1Nstats int
var mk1StatNames []string // w/ space, period, and dash replacement

// record last stat value in a nonsync map b/c it's single threaded accessed
var mk1LastStats map[xeth.Xid][]uint64

func mk1XethInit() {
	r := strings.NewReplacer(" ", "-", ".", "-", "_", "-")
	mk1Nstats = len(EthtoolStatNames)
	mk1StatNames = make([]string, mk1Nstats)
	for i, statName := range EthtoolStatNames {
		mk1StatNames[i] = r.Replace(statName)
	}
	mk1LastStats = make(map[xeth.Xid][]uint64)
	vnet.RunInitHooks.Add(fe1xeth.VnetRunInit)
	fe1xeth.AddRxMsgHook(mk1RxMsg)
}

func mk1RxMsg(msg interface{}) {
	if _, ok := msg.(xeth.Break); ok {
		if mk1XethBreaks++; mk1XethBreaks == 1 {
			mk1InitFe1Attrs()
			// FIXME=XETH
			// we should delay ready until fe1 has made the post
			mk1Ready()
			return
		}
	}
	if mk1XethBreaks == 0 {
		return
	}
	switch t := msg.(type) {
	case xeth.DevNew:
	case xeth.DevDel:
	case xeth.DevUp:
	case xeth.DevDown:
	case xeth.DevDump:
	case xeth.DevUnreg:
	case xeth.DevReg:
	case *xeth.DevEthtoolFlags:
		mk1EthtoolFlags(t.Xid, t.EthtoolFlagBits)
	default:
		_ = t
	}
}

func mk1InitFe1Attrs() {
	var ports []xeth.Xid
	for port := 0; port < NPorts; port++ {
		portId := TopPortVid - port
		portXid := xeth.Xid(portId)
		portAttrs := portXid.Attrs()
		portName := portAttrs.IfInfoName()
		portFe1Xid := fe1xeth.Xid{portXid}
		portFe1Attrs := portFe1Xid.Attrs()
		portFe1Attrs.Port(port)
		portAttrs.StatNames(mk1StatNames)
		portAttrs.Stats(make([]uint64, len(mk1StatNames)))
		mk1WriteStatNames(portName)
		mk1LastStats[portXid] = make([]uint64, mk1Nstats)
		mk1EthtoolFlags(portXid, portAttrs.EthtoolFlags())
		ports = append(ports, portXid)
		for subport := 1; subport < NSubPorts; subport++ {
			subId := TopPortVid - (subport * NPorts) - port
			subXid := xeth.Xid(subId)
			if !subXid.Valid() {
				break
			}
			if subport == 1 {
				// only set SubPort of port if there are
				// subports
				portFe1Attrs.SubPort(0)
			}
			subFe1Xid := fe1xeth.Xid{subXid}
			subAttrs := subXid.Attrs()
			subName := subAttrs.IfInfoName()
			subAttrs.StatNames(mk1StatNames)
			subAttrs.Stats(make([]uint64, len(mk1StatNames)))
			mk1WriteStatNames(subName)
			mk1LastStats[subXid] = make([]uint64, mk1Nstats)
			subFe1Attrs := subFe1Xid.Attrs()
			subFe1Attrs.Port(port)
			subFe1Attrs.SubPort(subport)
			subports := portFe1Attrs.SubPorts()
			subports = append(subports, subFe1Xid)
			portFe1Attrs.SubPorts(subports)
			mk1EthtoolFlags(subXid, subAttrs.EthtoolFlags())
			ports = append(ports, subXid)
		}
	}
	go mk1GoPubStats(ports)
	for port := 0; port < NPorts; port++ {
		portId := TopPortVid - port
		portXid := xeth.Xid(portId)
		portSpeed := portXid.Attrs().EthtoolSpeed()
		portFe1Xid := fe1xeth.Xid{portXid}
		portFe1Attrs := portFe1Xid.Attrs()
		portFe1Attrs.Bandwidth(vnet.Bandwidth(portSpeed * vnet.Mbps))
		subports := portFe1Attrs.SubPorts()
		var portlanes, sublanes int
		var mask uint
		switch portSpeed {
		case 100000, 40000:
			portlanes = 4
			subports = subports[:0]
			portFe1Attrs.SubPorts(subports)
		case 50000:
			portlanes = 2
			sublanes = 2
		case 25000, 20000, 10000, 1000:
			portlanes = 1
			sublanes = 1
		case 0:
			switch len(subports) {
			case 0:
				portlanes = 4
			case 1:
				portlanes = 2
				sublanes = 2
			case 3:
				portlanes = 1
				sublanes = 1
			}
		}
		portFe1Attrs.Lanes(portlanes)
		portFe1Attrs.LaneMask((1 << uint(portlanes)) - 1)
		if sublanes > 0 && len(subports) > 0 {
			if len(subports) == 1 {
				// shift subport index of xeth*-1
				subId := TopPortVid - NPorts - port
				subXid := xeth.Xid(subId)
				subFe1Xid := fe1xeth.Xid{subXid}
				subFe1Attrs := subFe1Xid.Attrs()
				subFe1Attrs.SubPort(2)
				subFe1Attrs.Lanes(sublanes)
				mask = ((1 << uint(sublanes)) - 1) << 2
				subFe1Attrs.LaneMask(mask)
			} else {
				for i, subFe1Xid := range subports {
					subi := uint(i + 1)
					subFe1Attrs := subFe1Xid.Attrs()
					mask = (1 << uint(sublanes)) - 1
					mask <<= subi
					subFe1Attrs.LaneMask(mask)
					subFe1Attrs.Lanes(sublanes)
					subAttrs := subFe1Xid.Xid.Attrs()
					subSpeed := subAttrs.EthtoolSpeed()
					subBW := vnet.Bandwidth(subSpeed)
					subBW *= vnet.Mbps
					subFe1Attrs.Bandwidth(subBW)
				}
			}
		}
	}
	xeth.Range(func(xid xeth.Xid) bool {
		fe1Xid := fe1xeth.Xid{xid}
		fe1Attrs := fe1Xid.Attrs()
		fe1Attrs.PuntIndex(int(xid) & 1)
		vlan := uint16((xid & XidPortMask) / xeth.VlanNVid)
		if vlan != 0 {
			fe1Attrs.Vlan(vlan)
			portXid := xid & xeth.VlanVidMask
			portFe1Xid := fe1xeth.Xid{portXid}
			portFe1Attrs := portFe1Xid.Attrs()
			vlans := portFe1Attrs.Vlans()
			portFe1Attrs.Vlans(append(vlans, fe1Xid))
			fe1Attrs.Port(portFe1Attrs.Port())
		}
		return true
	})
}

func mk1EthtoolFlags(xid xeth.Xid, bits xeth.EthtoolFlagBits) {
	fe1xid := fe1xeth.Xid{xid}
	attrs := fe1xid.Attrs()
	attrs.IsCr(bits.Test(CopperBit))
	attrs.IsFec74(bits.Test(Fec74Bit))
	attrs.IsFec91(bits.Test(Fec91Bit))
	attrs.IsKr(bits.Test(KrBit))
}

func mk1WriteStatNames(devName string) {
	for i, statName := range mk1StatNames {
		fn := fmt.Sprintf("/sys/class/net/%s/ethtool-stat-names/%03d",
			devName, i)
		if f, err := os.Open(fn); err == nil {
			f.WriteString(statName)
			f.Close()
			mk1.pubch <- fmt.Sprint(devName, ".", statName, ": 0")
		}
	}
}

func mk1GoPubStats(ports []xeth.Xid) {
	vnet.WG.Add(1)
	defer vnet.WG.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-vnet.StopCh:
			return
		case <-ticker.C:
			for _, xid := range ports {
				devName := xid.Attrs().IfInfoName()
				last := mk1LastStats[xid]
				current := xid.Attrs().Stats()
				for i := 0; i < mk1Nstats; i++ {
					ptr := &current[i]
					val := atomic.LoadUint64(ptr)
					if last[i] != val {
						last[i] = val
						s := fmt.Sprint(devName, ".",
							mk1StatNames[i], ": ",
							val)
						mk1.pubch <- s
					}
				}
			}
		}

	}
}
