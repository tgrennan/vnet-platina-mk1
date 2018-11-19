// Copyright 2016 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build kafka

package main

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/platinasystems/vnet"
	"os"
	"regexp"
	"time"
)

const hfChanDepth = 64

type HfInfo struct {
	vnet.Event
	mk1          *Mk1
	hwInterfaces ifStatsPollerInterfaceVec
	producer     *kafka.Producer

	pubch        chan string
	sequence     uint
	pollInterval float64 // pollInterval in milliseconds
	msgCount     uint64
	hostname     string
}

func (hf *HfInfo) Init(mk1 *Mk1) {
	hf.pubch = make(chan string, hfChanDepth)
	hf.mk1 = mk1
	hf.pollInterval = defaultFastPollIntervalMilliSec
	hf.hostname, _ = os.Hostname()
	hf.addEvent(0)
	go hf.gopublishHf()
}

func (hf *HfInfo) Close() {
	if hf.pubch != nil {
		close(hf.pubch)
	}
}

func (hf *HfInfo) setPollInterval(val float64) {
	hf.pollInterval = val
}
func (hf *HfInfo) initProducer(broker string) {
	var err error
	if hf.producer != nil {
		hf.producer.Close()
	}
	hf.producer, err = kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": broker})
	if err != nil {
		fmt.Errorf("error while creating producer: %v", err)
	} else {
		go func() {
			for e := range hf.producer.Events() {
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
func (hf *HfInfo) gopublishHf() {
	topic := "hf-counters"
	for s := range hf.pubch {
		hf.producer.ProduceChannel() <- &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(s),
		}
		hf.msgCount++
	}
}

func (hf *HfInfo) publish(data map[string]string) {
	for k, v := range data {
		hf.pubch <- fmt.Sprintf("%s,%d,%s,%s", hf.hostname, time.Now().UnixNano()/1000000, k, v)
	}
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

func (hf *HfInfo) addEvent(dt float64) { hf.mk1.vnet.SignalEventAfter(hf, dt) }
func (hf *HfInfo) String() string {
	return fmt.Sprintf("Kafka stats poller sequence %d", hf.sequence)
}
func (hf *HfInfo) EventAction() {
	// Schedule next event in 200 milliseconds; do before fetching counters so that time interval is accurate.
	hf.addEvent(hf.pollInterval / 1000)

	// Publish all sw/hw interface counters even with zero values for first poll.
	// This was all possible counters have valid values in redis.
	// Otherwise only publish to redis when counter values change.
	var c = make(map[string]string)
	hf.mk1.vnet.ForeachHighFreqHwIfCounter(true, hf.mk1.unixInterfacesOnly,
		func(hi vnet.Hi, counter string, value uint64) {
			ifname := hi.Name(&hf.mk1.vnet)
			hf.hwInterfaces.Validate(uint(hi))
			delta, _ := hf.hwInterfaces[hi].updateHf(counter, value)
			c[ifname] = c[ifname] + fmt.Sprint(delta) + ","
		})
	if hf.producer != nil {
		hf.publish(c)
	}
	hf.sequence++
}
