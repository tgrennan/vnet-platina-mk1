// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build ignore

package main

import (
	"fmt"

	"github.com/platinasystems/elib/parse"
	"github.com/platinasystems/vnet"
	"github.com/platinasystems/vnet/ethernet"
)

type event struct {
	vnet.Event
	in           parse.Input
	key, value   string
	err          chan error
	newValue     chan string
	isReadyEvent bool
}

func (e *event) String() string {
	return fmt.Sprintf("redis set %s = %s", e.key, e.value)
}

func (e *event) EventAction() {
	var (
		hi     vnet.Hi
		si     vnet.Si
		bw     vnet.Bandwidth
		enable parse.Enable
		media  string
		itv    float64
		fec    ethernet.ErrorCorrectionType
		addr   string
	)
	if e.isReadyEvent {
		Mk1.poller.pubch <- fmt.Sprint(e.key, ": ", e.value)
		return
	}
	e.in.Init(nil)
	e.in.Add(e.key, e.value)
	v := &Mk1.vnet
	switch {
	case e.in.Parse("%v.speed %v", &hi, v, &bw):
		{
			err := hi.SetSpeed(v, bw)
			h := v.HwIf(hi)
			if err == nil {
				e.newValue <- h.Speed().String()
			}
			e.err <- err
		}
	case e.in.Parse("%v.admin %v", &si, v, &enable):
		{
			err := si.SetAdminUp(v, bool(enable))
			es := "false"
			if bool(enable) {
				es = "true"
			}
			if err == nil {
				e.newValue <- es
			}
			e.err <- err
		}
	case e.in.Parse("%v.media %s", &hi, v, &media):
		{
			err := hi.SetMedia(v, media)
			h := v.HwIf(hi)
			if err == nil {
				e.newValue <- h.Media()
			}
			e.err <- err
		}
	case e.in.Parse("%v.fec %v", &hi, v, &fec):
		{
			err := ethernet.SetInterfaceErrorCorrection(v, hi, fec)
			if err == nil {
				if h, ok := v.HwIfer(hi).(ethernet.HwInterfacer); ok {
					e.newValue <- h.GetInterface().ErrorCorrectionType.String()
				} else {
					err = fmt.Errorf("error setting fec")
				}
			}
			e.err <- err
		}
	case e.in.Parse("pollInterval %f", &itv):
		if itv < 1 {
			e.err <- fmt.Errorf("pollInterval must be 1 second or longer")
		} else {
			Mk1.poller.pollInterval = itv
			e.newValue <- fmt.Sprintf("%f", itv)
			e.err <- nil
		}
	case e.in.Parse("pollInterval.msec %f", &itv):
		if itv < 1 {
			e.err <- fmt.Errorf("pollInterval.msec must be 1 millisecond or longer")
		} else {
			Mk1.fastPoller.pollInterval = itv
			e.newValue <- fmt.Sprintf("%f", itv)
			e.err <- nil
		}
	case e.in.Parse("kafka-broker %s", &addr):
		Mk1.initProducer(addr)
		e.newValue <- fmt.Sprintf("%s", addr)
		e.err <- nil
	case e.in.Parse("unresolved-arpInterval %f", &itv):
		if itv < 1 {
			e.err <- fmt.Errorf("unresolvedArpInterval must be 1 second or longer")
		} else {
			Mk1.unresolvedArper.pollInterval = itv
			e.newValue <- fmt.Sprintf("%f", itv)
			e.err <- nil
		}
	default:
		e.err <- fmt.Errorf("can't set %s to %v", e.key, e.value)
	}
	Mk1.eventPool.Put(e)
}
