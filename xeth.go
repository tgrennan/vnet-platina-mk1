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
	"github.com/platinasystems/vnet/devices/vnetxeth"
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
	vnet.RunInitHooks.Add(vnetxeth.RunInit)
	vnetxeth.AddRxMsgHook(mk1RxMsg)
	vnetxeth.DetailAttr("bandwidth", fe1xeth.BandwidthAttr)
	vnetxeth.DetailAttr("copper", fe1xeth.IsCrAttr)
	vnetxeth.DetailAttr("fec74", fe1xeth.IsFec74Attr)
	vnetxeth.DetailAttr("fec91", fe1xeth.IsFec91Attr)
	vnetxeth.DetailAttr("backplane", fe1xeth.IsKrAttr)
	vnetxeth.DetailAttr("lane-mask", fe1xeth.LaneMaskAttr)
	vnetxeth.DetailAttr("lanes", fe1xeth.LanesAttr)
	vnetxeth.DetailAttr("port", fe1xeth.PortAttr)
	vnetxeth.DetailAttr("punt-index", fe1xeth.PuntIndexAttr)
	vnetxeth.DetailAttr("subport", fe1xeth.SubPortAttr)
	vnetxeth.DetailAttr("subports", fe1xeth.SubPortsAttr)
	vnetxeth.DetailAttr("vlan", fe1xeth.VlanAttr)
	vnetxeth.DetailAttr("vlans", fe1xeth.VlansAttr)
	vnetxeth.Less = mk1Less
}

func mk1Less(ixid, jxid xeth.Xid) bool {
	// this lists in this order,
	//	xeth[1:32][-1]
	//	xeth[1:32][-1].[1:4904]
	//	...
	//	xeth[1:32]-2
	//	xeth[1:32]-2.[1:4904]
	//	...
	//	xeth[1:32]-3
	//	xeth[1:32]-3.[1:4904]
	//	...
	//	xeth[1:32]-4
	//	xeth[1:32]-4.[1:4904]
	//	...
	//	xethbr[0:4094]
	//	...
	//	xethlag[0:4094]
	//	...
	vidIsPort := func(vid uint16) bool {
		return BottomPortVid <= vid && vid <= TopPortVid
	}
	ivid := uint16(ixid & xeth.VlanVidMask)
	jvid := uint16(jxid & xeth.VlanVidMask)
	ivlan := uint16(ixid / xeth.VlanNVid)
	jvlan := uint16(jxid / xeth.VlanNVid)
	iIsPort := vidIsPort(ivid)
	jIsPort := vidIsPort(jvid)
	if iIsPort {
		if jIsPort {
			if ivid == jvid {
				return ivlan < jvlan
			}
			return ivid > jvid
		}
		return true
	}
	if jIsPort {
		return false
	}
	return ixid.Attrs().IfInfoName() < jxid.Attrs().IfInfoName()
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
			portFe1Attrs.SubPort(0)
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
		var lanes int
		switch portSpeed {
		case 100000, 40000:
			lanes = 4
		case 50000:
			lanes = 2
		case 25000, 20000, 10000, 1000:
			lanes = 1
		case 0:
			switch len(subports) {
			case 0:
				lanes = 4
			case 1:
				lanes = 2
			case 3:
				lanes = 1
			}
		}
		portFe1Attrs.Lanes(lanes)
		portFe1Attrs.LaneMask((1 << uint(lanes)) - 1)
		if len(subports) > 0 {
			if len(subports) == 1 {
				// shift subport index of xeth*-1
				subId := TopPortVid - NPorts - port
				subXid := xeth.Xid(subId)
				subFe1Xid := fe1xeth.Xid{subXid}
				subFe1Attrs := subFe1Xid.Attrs()
				subFe1Attrs.SubPort(2)
			} else {
				for i, subFe1Xid := range subports {
					subi := i + 1
					subFe1Attrs := subFe1Xid.Attrs()
					mask := uint(((1 << uint(lanes)) - 1) <<
						uint(subi))
					subFe1Attrs.LaneMask(mask)
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