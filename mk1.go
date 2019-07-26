// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"os/exec"

	"github.com/platinasystems/elib/parse"
	"github.com/platinasystems/fe1/pc"
	"github.com/platinasystems/redis"
	"github.com/platinasystems/redis/publisher"
	"github.com/platinasystems/vnet"
	"github.com/platinasystems/vnet/devices/bus/pci"
	"github.com/platinasystems/vnet/devices/vnetonie"
)

const FIXME = false

var mk1 = struct {
	pubch chan<- string
	/* FIXME-XETH
	eventPool sync.Pool
	platform  fe1.Fe1Platform
	poller          ifStatsPoller
	fastPoller      fastIfStatsPoller
	unresolvedArper unresolvedArper
	producer	*kafka.Producer

	// Enable publish of Non-unix (e.g. non-tuntap) interfaces.
	// This will include all vnet interfaces.
	unixInterfacesOnly bool

	prevHwIfConfig map[string]*hwIfConfig
	FIXME-XETH */
}{
	/* FIXME-XETH
	eventPool: sync.Pool{
		New: func() interface{} {
			return &event{
				err:      make(chan error, 1),
				newValue: make(chan string, 1),
			}
		},
	},
	FIXME-XETH */
}

/* FIXME-XETH
type hwIfConfig struct {
	speed string
	media string
	fec   string
}
FIXME-XETH */

func mk1Main() error {
	var in parse.Input

	in.SetString("cli { listen { no-prompt socket @vnet } }")

	err := mk1GpioInit()
	if err != nil {
		return err
	}

	vnet.Init()

	redis.DefaultHash = "platina-mk1"
	err = redis.IsReady()
	if err != nil {
		return err
	}

	pub, err := publisher.New()
	if err != nil {
		return err
	}

	pubch := make(chan string, 1<<16)
	mk1.pubch = pubch

	go mk1GoPub(pub, pubch)

	mk1OnieInit()
	mk1XethInit()
	mk1VnetInit()

	exec.Command("ip", "-a", "neighbor", "flush", "all").Run()

	return vnet.Run(&in)
}

func mk1Ready() {
	mk1.pubch <- "ready: true"
}

func mk1GoPub(pub *publisher.Publisher, pubch <-chan string) {
	vnet.WG.Add(1)
	defer vnet.WG.Done()
	defer pub.Close()
	for {
		select {
		case <-vnet.StopCh:
			return
		case s, ok := <-pubch:
			if !ok {
				return
			}
			pub.Print("vnet.", s)
		}
	}
}

func mk1GpioInit() error {
	gpio := pca9535_main{
		bus_index:   0,
		bus_address: 0x74,
	}
	if pc.EnableGpioLedOutput {
		if err := gpio.do(gpio.led_output_enable); err != nil {
			return err
		}
	}
	if pc.EnableGpioSwitchReset {
		if err := gpio.do(gpio.switch_reset); err != nil {
			return err
		}
	}
	return nil
}

func mk1OnieInit() {
	// Alpha level board (version 0):
	//   No lane remapping, but the MK1 front panel ports are flipped and 0-based.
	// Beta & Production level boards have version 1 and above:
	//   No lane remapping, but the MK1 front panel ports are flipped and 1-based.

	vnet.RunInitHooks.Add(vnetonie.RunInit)
	vnet.RunInitHooks.Add(func() {
		if vnetonie.DeviceVersion == 0 {
			pc.PortNumberOffset = 0
		}
	})
}

func mk1VnetInit() {
	/* FIXME-XETH
	v := &Mk1.vnet
	p := &Mk1.platform
	m4 := ip4.Init(v)
	m6 := ip6.Init(v)
	gre.Init(v)
	ethernet.Init(v, m4, m6)
	FIXME-XETH */

	pci.Init()

	/* FIXME-XETH
	pg.Init(v)
	ipcli.Init(v)
	unix.Init(v, unix.Config{RxInjectNodeName: "fe1-cpu"})

	if true {
		qsfpInit()
	}

	fe1.Init(v, p)
	fe1a.RegisterDeviceIDs(v)
	fe1.AddPlatform(v, p)

	FIXME-XETH */
}

/* FIXME-XETH
func mk1Init() {
	const (
		defaultPollInterval             = 5
		defaultFastPollIntervalMilliSec = 200
		defaultUnresolvedArpInterval    = 1
	)
	Mk1.poller.mk1 = mk1
	Mk1.fastPoller.mk1 = mk1
	Mk1.unresolvedArper.mk1 = mk1

	Mk1.poller.addEvent(0)
	Mk1.fastPoller.addEvent(0)
	Mk1.unresolvedArper.addEvent(0)

	Mk1.poller.pollInterval = defaultPollInterval
	Mk1.fastPoller.pollInterval = defaultFastPollIntervalMilliSec
	Mk1.unresolvedArper.pollInterval = defaultUnresolvedArpInterval

	Mk1.fastPoller.hostname, _ = os.Hostname()
	mk1PubHwIfConfig()
	mk1Set("ready", "true", true)

	Mk1.poller.pubch <- fmt.Sprint("poll.max-channel-depth: ", chanDepth)
	Mk1.poller.pubch <- fmt.Sprint("pollInterval: ", defaultPollInterval)
	Mk1.poller.pubch <- fmt.Sprint("pollInterval.msec: ",
		defaultFastPollIntervalMilliSec)
	Mk1.poller.pubch <- fmt.Sprint("kafka-broker: ", "")
	Mk1.poller.pubch <- fmt.Sprint("unresolved-arpInterval: ", defaultUnresolvedArpInterval)
}

func mk1PublishLink(hi vnet.Hi, isUp bool) {
	Mk1.poller.pubch <- fmt.Sprint(hi.Name(&mk1.vnet), ".link: ",
		parse.Enable(isUp))
}

func mk1HwIfLinkUpDown(v *vnet.Vnet, hi vnet.Hi, isUp bool) error {
	if mk1HwIsOk(hi) {
		hw := Mk1.vnet.HwIf(hi)
		x := hw.X.(*mk1.X)
		xeth.SetCarrier(x.Xid, isUp)
		mk1PublishLink(hi, isUp)
	}
	return nil
}

func mk1GoPublishHf() {
	topic := "hf-counters"
	for s := range Mk1.fastPoller.pubch {
		_ = s
		_ = topic
		Mk1.producer.ProduceChannel() <- &kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &topic,
				Partition: kafka.PartitionAny,
			},
			Value: []byte(s),
		}
		Mk1.fastPoller.msgCount++
	}
}

func mk1InitProducer(broker string) {
	var err error
	if Mk1.producer != nil {
		Mk1.producer.Close()
	}
	Mk1.producer, err = kafka.NewProducer(&kafka.ConfigMap{
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
}
FIXME-XETH */
