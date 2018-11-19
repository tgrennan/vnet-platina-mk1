// Copyright 2016 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build !kafka

package main

type HfInfo struct {
}

func (hf *HfInfo) Init(mk1 *Mk1) {}

func (hf *HfInfo) initProducer(broker string) {}

func (hf *HfInfo) Close() {}

func (hf *HfInfo) setPollInterval(val float64) {}
