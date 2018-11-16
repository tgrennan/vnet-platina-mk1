// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/platinasystems/atsock"
	"github.com/platinasystems/elib/parse"
	"github.com/platinasystems/redis"
	"github.com/platinasystems/redis/publisher"
	"github.com/platinasystems/redis/rpc/args"
	"github.com/platinasystems/redis/rpc/reply"
	"github.com/platinasystems/vnet"
	"github.com/platinasystems/vnet/ethernet"
	vnetfe1 "github.com/platinasystems/vnet/platforms/fe1"
	vnetmk1 "github.com/platinasystems/vnet/platforms/mk1"
	"github.com/platinasystems/vnet/unix"
	"github.com/platinasystems/xeth"
	yaml "gopkg.in/yaml.v2"
)

const chanDepth = 1 << 16

type Mk1 struct {
	vnet       vnet.Vnet
	platform   vnetfe1.Platform
	eventPool  sync.Pool
	poller     ifStatsPoller
	fastPoller fastIfStatsPoller
	pub        *publisher.Publisher
	// producer	*kafka.Producer

	// Enable publish of Non-unix (e.g. non-tuntap) interfaces.
	// This will include all vnet interfaces.
	unixInterfacesOnly bool

	prevHwIfConfig map[string]*hwIfConfig
}

type hwIfConfig struct {
	speed string
	media string
	fec   string
}

func mk1Main() error {
	const nports = 4 * 32
	const ncounters = 512

	var mk1 Mk1
	var in parse.Input

	xeth.EthtoolPrivFlagNames = flags
	xeth.EthtoolStatNames = stats

	eth1, err := net.InterfaceByName("eth1")
	if err != nil {
		return err
	}
	eth2, err := net.InterfaceByName("eth2")
	if err != nil {
		return err
	}

	if err = redis.IsReady(); err != nil {
		return err
	}

	if mk1.pub, err = publisher.New(); err != nil {
		return err
	}
	defer mk1.pub.Close()

	if err = xeth.Start(redis.DefaultHash); err != nil {
		return err
	}

	rpc.Register(&mk1)

	sock, err := atsock.NewRpcServer("vnetd")
	if err != nil {
		return err
	}
	defer sock.Close()

	mk1.poller.pubch = make(chan string, chanDepth)
	defer close(mk1.poller.pubch)
	go mk1.gopublish()

	if false {
		mk1.fastPoller.pubch = make(chan string)
		defer close(mk1.fastPoller.pubch)
		go mk1.gopublishHf()
	}

	err = redis.Assign(redis.DefaultHash+":vnet.", "vnetd", "Mk1")
	if err != nil {
		return err
	}

	vnet.PortIsCopper = func(ifname string) bool {
		if p, found := vnet.Ports[ifname]; found {
			return p.Flags.Test(CopperBit)
		}
		return false
	}
	vnet.PortIsFec74 = func(ifname string) bool {
		if p, found := vnet.Ports[ifname]; found {
			return p.Flags.Test(Fec74Bit)
		}
		return false
	}
	vnet.PortIsFec91 = func(ifname string) bool {
		if p, found := vnet.Ports[ifname]; found {
			return p.Flags.Test(Fec91Bit)
		}
		return false
	}

	xeth.DumpIfinfo()
	err = xeth.UntilBreak(func(buf []byte) error {
		ptr := unsafe.Pointer(&buf[0])
		kind := xeth.KindOf(buf)
		switch kind {
		case xeth.XETH_MSG_KIND_ETHTOOL_FLAGS:
			msg := (*xeth.MsgEthtoolFlags)(ptr)
			xethif := xeth.Interface.Indexed(msg.Ifindex)
			ifname := xethif.Ifinfo.Name
			entry, found := vnet.Ports[ifname]
			if found {
				entry.Flags = xeth.EthtoolPrivFlags(msg.Flags)
				dbgSvi.Log(ifname, entry.Flags)
			}
		case xeth.XETH_MSG_KIND_ETHTOOL_SETTINGS:
			msg := (*xeth.MsgEthtoolSettings)(ptr)
			xethif := xeth.Interface.Indexed(msg.Ifindex)
			ifname := xethif.Ifinfo.Name
			entry, found := vnet.Ports[ifname]
			if found {
				entry.Speed = xeth.Mbps(msg.Speed)
				dbgSvi.Log(ifname, entry.Speed)
			}
		case xeth.XETH_MSG_KIND_IFINFO:
			var punt_index uint8
			msg := (*xeth.MsgIfinfo)(ptr)

			// convert eth1/eth2 to meth-0/meth-1
			switch msg.Iflinkindex {
			case int32(eth1.Index):
				punt_index = 0
			case int32(eth2.Index):
				punt_index = 1
			}

			switch msg.Devtype {
			case xeth.XETH_DEVTYPE_LINUX_VLAN:
				fallthrough
			case xeth.XETH_DEVTYPE_LINUX_BRIDGE:
				fallthrough
			case xeth.XETH_DEVTYPE_XETH_PORT:
				err = unix.ProcessInterfaceInfo((*xeth.MsgIfinfo)(ptr), vnet.PreVnetd, nil, punt_index)
			case xeth.XETH_DEVTYPE_LINUX_UNKNOWN:
				// FIXME
			}
		case xeth.XETH_MSG_KIND_IFA:
			err = unix.ProcessInterfaceAddr((*xeth.MsgIfa)(ptr), vnet.PreVnetd, nil)
		}
		dbgSvi.Log(err)
		return nil
	})
	if err != nil {
		return err
	}

	for ifname, entry := range vnet.Ports {
		dbgSvi.Log(ifname, "flags", entry.Flags)
		dbgSvi.Log(ifname, "speed", entry.Speed)
	}

	mk1.eventPool.New = mk1.newEvent

	mk1.vnet.RegisterHwIfAddDelHook(mk1.hw_if_add_del)
	mk1.vnet.RegisterHwIfLinkUpDownHook(mk1.hw_if_link_up_down)
	mk1.vnet.RegisterSwIfAddDelHook(mk1.sw_if_add_del)
	mk1.vnet.RegisterSwIfAdminUpDownHook(mk1.sw_if_admin_up_down)

	if err = mk1.setup(); err != nil {
		return err
	}

	in.SetString("cli { listen { no-prompt socket @vnet } }")

	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)

	sigterm := make(chan os.Signal)
	signal.Notify(sigterm, syscall.SIGTERM)
	defer close(sigterm)
	go func() {
		for sig := range sigterm {
			if sig == syscall.SIGTERM {
				mk1.vnet.Quit()
			}
		}
	}()

	err = mk1.vnet.Run(&in)

	begin := time.Now()
	exerr := vnetmk1.PlatformExit(&mk1.vnet, &mk1.platform)
	if err == nil {
		err = exerr
	}
	dbgVnetd.Log("stopped in", time.Now().Sub(begin))

	begin = time.Now()
	xeth.Stop()
	dbgVnetd.Log("xeth closeed in", time.Now().Sub(begin))

	return err
}

func (mk1 *Mk1) Hset(args args.Hset, reply *reply.Hset) error {
	field := strings.TrimPrefix(args.Field, "vnet.")
	err := mk1.set(field, string(args.Value), false)
	if err == nil {
		*reply = 1
	}
	return err
}

func (mk1 *Mk1) init() {
	const (
		defaultPollInterval             = 5
		defaultFastPollIntervalMilliSec = 200
	)
	mk1.poller.mk1 = mk1
	mk1.fastPoller.mk1 = mk1
	mk1.poller.addEvent(0)
	mk1.fastPoller.pollInterval = defaultFastPollIntervalMilliSec
	mk1.fastPoller.addEvent(0)
	mk1.poller.pollInterval = defaultPollInterval
	mk1.fastPoller.hostname, _ = os.Hostname()
	mk1.pubHwIfConfig()
	mk1.set("ready", "true", true)
	mk1.poller.pubch <- fmt.Sprint("poll.max-channel-depth: ", chanDepth)
	mk1.poller.pubch <- fmt.Sprint("pollInterval: ", defaultPollInterval)
	mk1.poller.pubch <- fmt.Sprint("pollInterval.msec: ",
		defaultFastPollIntervalMilliSec)
	mk1.poller.pubch <- fmt.Sprint("kafka-broker: ", "")
}

func (mk1 *Mk1) newEvent() interface{} {
	return &event{
		mk1:      mk1,
		err:      make(chan error, 1),
		newValue: make(chan string, 1),
	}
}

func (mk1 *Mk1) hw_is_ok(hi vnet.Hi) bool {
	h := mk1.vnet.HwIfer(hi)
	hw := mk1.vnet.HwIf(hi)
	if !hw.IsProvisioned() {
		return false
	}
	return !mk1.unixInterfacesOnly || h.IsUnix()
}

func (mk1 *Mk1) sw_is_ok(si vnet.Si) bool {
	h := mk1.vnet.HwIferForSupSi(si)
	return h != nil && mk1.hw_is_ok(h.GetHwIf().Hi())
}

func (mk1 *Mk1) sw_if_add_del(v *vnet.Vnet, si vnet.Si, isDel bool) error {
	mk1.sw_if_admin_up_down(v, si, false)
	return nil
}

func (mk1 *Mk1) sw_if_admin_up_down(v *vnet.Vnet, si vnet.Si, isUp bool) error {
	if mk1.sw_is_ok(si) {
		mk1.poller.pubch <- fmt.Sprint(si.Name(v), ".admin: ",
			parse.Enable(isUp))
	}
	return nil
}

func (mk1 *Mk1) publish_link(hi vnet.Hi, isUp bool) {
	mk1.poller.pubch <- fmt.Sprint(hi.Name(&mk1.vnet), ".link: ",
		parse.Enable(isUp))
}

func (mk1 *Mk1) hw_if_add_del(v *vnet.Vnet, hi vnet.Hi, isDel bool) error {
	mk1.hw_if_link_up_down(v, hi, false)
	return nil
}

func (mk1 *Mk1) hw_if_link_up_down(v *vnet.Vnet, hi vnet.Hi, isUp bool) error {
	if mk1.hw_is_ok(hi) {
		var flag uint8 = xeth.XETH_CARRIER_OFF
		if isUp {
			flag = xeth.XETH_CARRIER_ON
		}
		// Make sure interface is known to platina-mk1 driver
		if _, found := vnet.Ports[hi.Name(v)]; found {
			index := xeth.Interface.Named(hi.Name(v)).Ifinfo.Index
			xeth.Carrier(index, flag)
		}
		mk1.publish_link(hi, isUp)
	}
	return nil
}

func (mk1 *Mk1) parsePortConfig() (err error) {
	plat := &mk1.platform
	if false { // /etc/goes/portprovision
		filename := "/etc/goes/portprovision"
		source, err := ioutil.ReadFile(filename)
		// If no file PortConfig will be left empty and lower layers will default
		if err == nil {
			err = yaml.Unmarshal(source, &plat.PortConfig)
			if err != nil {
				dbgSvi.Log(err)
				panic(err)
			}
			for _, p := range plat.PortConfig.Ports {
				dbgSvi.Log("Provision", p.Name,
					"speed", p.Speed,
					"lanes", p.Lanes,
					"count", p.Count)
			}
		}
	} else { // ethtool
		// Massage ethtool port-provision format into fe1 format
		var pp vnetfe1.PortProvision
		for ifname, entry := range vnet.Ports {
			if entry.Devtype >= xeth.XETH_DEVTYPE_LINUX_UNKNOWN {
				continue
			}
			pp.Name = ifname
			pp.Portindex = entry.Portindex
			pp.Subportindex = entry.Subportindex
			pp.Vid = ethernet.VlanTag(entry.Vid)
			pp.PuntIndex = entry.PuntIndex
			pp.Speed = fmt.Sprintf("%dg", entry.Speed/1000)
			// Need some more help here from ethtool to disambiguate
			// 40G 2-lane and 40G 4-lane
			// 20G 2-lane and 20G 1-lane
			// others?
			dbgSvi.Logf("From ethtool: name %v entry %+v pp %+v",
				ifname, entry, pp)
			pp.Count = 1
			switch entry.Speed {
			case 100000, 40000:
				pp.Lanes = 4
			case 50000:
				pp.Lanes = 2
			case 25000, 20000, 10000, 1000:
				pp.Lanes = 1
			case 0: // need to calculate autoneg defaults
				pp.Lanes =
					mk1.getDefaultLanes(uint(pp.Portindex),
						uint(pp.Subportindex))
			}

			// entry is what vnet sees; pp is what gets configured into fe1
			// 2-lanes ports, e.g. 50g-ports, must start on subport index 0 or 2 in fe1
			// Note number of subports per port can only be 1, 2, or 4; and first subport must start on subport index 0
			if pp.Lanes == 2 {
				switch entry.Subportindex {
				case 0:
					//OK
				case 1:
					//shift index for fe1
					pp.Subportindex = 2
				case 2:
					//OK
				default:
					dbgVnetd.Log(ifname,
						"has invalid subport index",
						entry.Subportindex)

				}
			}

			plat.PortConfig.Ports = append(plat.PortConfig.Ports, pp)
		}
	}
	return
}

func (mk1 *Mk1) parseBridgeConfig() (err error) {
	plat := &mk1.platform

	if plat.BridgeConfig.Bridges == nil {
		plat.BridgeConfig.Bridges =
			make(map[ethernet.VlanTag]*vnetfe1.BridgeProvision)
	}

	// for each bridge entry, create bridge config
	for vid, entry := range vnet.Bridges {
		bp, found := plat.BridgeConfig.Bridges[ethernet.VlanTag(vid)]
		if !found {
			bp = new(vnetfe1.BridgeProvision)
			plat.BridgeConfig.Bridges[ethernet.VlanTag(vid)] = bp
		}
		bp.PuntIndex = entry.PuntIndex
		bp.Addr = entry.Addr
		dbgSvi.Log("parse bridge", vid)
	}

	// for each bridgemember entry, add to pbm or ubm of matching bridge config
	for ifname, entry := range vnet.BridgeMembers {
		bp, found := plat.BridgeConfig.Bridges[ethernet.VlanTag(entry.Vid)]
		if found {
			if entry.IsTagged {
				bp.TaggedPortVids =
					append(bp.TaggedPortVids,
						ethernet.VlanTag(entry.PortVid))
			} else {
				bp.UntaggedPortVids =
					append(bp.UntaggedPortVids,
						ethernet.VlanTag(entry.PortVid))
			}
			dbgSvi.Log("bridgemember", ifname,
				"added to vlan", entry.Vid)
			dbgSvi.Logf("bridgemember %+v", bp)
		} else {
			dbgSvi.Log("bridgemember", ifname, "ignored, vlan",
				entry.Vid, "not found")
		}
	}
	return
}

func (*Mk1) parseFibConfig(v *vnet.Vnet) (err error) {
	// Process Interface addresses that have been learned from platina xeth driver
	// ip4IfaddrMsg(msg.Prefix, isDel)
	// Process Route data that have been learned from platina xeth driver
	// Since TH/Fp-ports are not initialized what could these be?
	//for _, fe := range vnet.FdbRoutes {
	//ip4IfaddrMsg(fe.Address, fe.Mask, isDel)
	//}
	return
}

func (mk1 *Mk1) pubHwIfConfig() {
	v := &mk1.vnet
	if mk1.prevHwIfConfig == nil {
		mk1.prevHwIfConfig = make(map[string]*hwIfConfig)
	}
	v.ForeachHwIf(mk1.unixInterfacesOnly, func(hi vnet.Hi) {
		h := v.HwIf(hi)
		ifname := hi.Name(v)
		speed := h.Speed().String()
		media := h.Media()
		entry, found := mk1.prevHwIfConfig[ifname]
		if !found {
			entry = new(hwIfConfig)
			mk1.prevHwIfConfig[ifname] = entry
		}
		if speed != mk1.prevHwIfConfig[ifname].speed {
			s := fmt.Sprint(ifname, ".speed: ", speed)
			mk1.prevHwIfConfig[ifname].speed = speed
			mk1.poller.pubch <- s
		}
		if media != mk1.prevHwIfConfig[ifname].media {
			s := fmt.Sprint(ifname, ".media: ", media)
			mk1.prevHwIfConfig[ifname].media = media
			mk1.poller.pubch <- s
		}
		if h, ok := v.HwIfer(hi).(ethernet.HwInterfacer); ok {
			fec := h.GetInterface().ErrorCorrectionType.String()
			if fec != mk1.prevHwIfConfig[ifname].fec {
				s := fmt.Sprint(ifname, ".fec: ", fec)
				mk1.prevHwIfConfig[ifname].fec = fec
				mk1.poller.pubch <- s
			}
		}
	})
}

func (mk1 *Mk1) set(key, value string, isReadyEvent bool) (err error) {
	e := mk1.eventPool.Get().(*event)
	e.key = key
	e.value = value
	e.isReadyEvent = isReadyEvent
	mk1.vnet.SignalEvent(e)
	if isReadyEvent {
		return
	}
	if err = <-e.err; err == nil {
		newValue := <-e.newValue
		mk1.poller.pubch <- fmt.Sprint(e.key, ": ", newValue)
	}
	return
}

func (mk1 *Mk1) setup() error {
	mk1.platform.Init = mk1.init

	s, err := redis.Hget(redis.DefaultHash, "eeprom.DeviceVersion")
	if err != nil {
		return err
	}
	if _, err = fmt.Sscan(s, &mk1.platform.Version); err != nil {
		return err
	}
	s, err = redis.Hget(redis.DefaultHash, "eeprom.NEthernetAddress")
	if err != nil {
		return err
	}
	if _, err = fmt.Sscan(s, &mk1.platform.NEthernetAddress); err != nil {
		return err
	}
	s, err = redis.Hget(redis.DefaultHash, "eeprom.BaseEthernetAddress")
	if err != nil {
		return err
	}
	input := new(parse.Input)
	input.SetString(s)
	mk1.platform.BaseEthernetAddress.Parse(input)

	fi, err := os.Stat("/sys/bus/pci/drivers/ixgbe")
	mk1.platform.KernelIxgbe = err == nil && fi.IsDir()

	mk1.unixInterfacesOnly = !mk1.platform.KernelIxgbe

	// Default to using MSI versus INTX for switch chip.
	mk1.platform.EnableMsiInterrupt = true

	// Get initial port config from platina-mk1
	mk1.parsePortConfig()
	mk1.parseBridgeConfig()

	return vnetmk1.PlatformInit(&mk1.vnet, &mk1.platform)
}

func (*Mk1) getDefaultLanes(port, subport uint) (lanes uint) {
	lanes = 1

	// Two cases covered:
	// * 4-lane
	//         if first subport of port and only subport in set number of lanes should be 4
	// * 2-lane
	//         if first and third subports of port are present then number of lanes should be 2
	//         Unfortunately, 2-lane autoneg doesn't work for TH but leave this code here
	//         for possible future chipsets.
	//

	numSubports, _ := subportsMatchingPort(port)
	switch numSubports {
	case 1:
		lanes = 4
	case 2:
		lanes = 2
	case 4:
		lanes = 1
	default:
		dbgVnetd.Log("port", port, "has invalid subports:",
			numSubports)
	}

	return
}

func (mk1 *Mk1) gopublish() {
	for s := range mk1.poller.pubch {
		mk1.pub.Print("vnet.", s)
	}
}

func (mk1 *Mk1) gopublishHf() {
	topic := "hf-counters"
	for s := range mk1.fastPoller.pubch {
		_ = s
		_ = topic
		/* FIXME
		mk1.producer.ProduceChannel() <- &kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &topic,
				Partition: kafka.PartitionAny,
			},
			Value: []byte(s),
		}
		mk1.fastPoller.msgCount++
		*/
	}
}

func (mk1 *Mk1) initProducer(broker string) {
	/* FIXME
	var err error
	if mk1.producer != nil {
		mk1.producer.Close()
	}
	mk1.producer, err = kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
	})
	if err != nil {
		fmt.Errorf("error while creating producer: %v", err)
	} else {
		go func() {
			for e := range i.producer.Events() {
				switch ev := e.(type) {
				case *kafka.Message:
					m := ev
					if m.TopicPartition.Error != nil {
						fmt.Errorf("Delivery of msg to topic %s [%d] at offset %v failed: %v \n",
							*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset, m.TopicPartition.Error)
					}
				default:
					fmt.Printf("Ignored event: %s\n", ev)
				}
			}
		}()
	}
	*/
}
