// Copyright © 2016-2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// FIXME-XETH replace this with kernel ucd9535 i2c driver

// +build !ignore

package main

import (
	"time"

	"github.com/platinasystems/i2c"
)

type pca9535_main struct {
	bus_index, bus_address int
}

func (m *pca9535_main) do(f func(bus *i2c.Bus) error) (err error) {
	return i2c.Do(m.bus_index, m.bus_address, f)
}

const (
	pca9535_reg_input_0  = iota // read-only input bits [7:0]
	pca9535_reg_input_1         // read-only input bits [15:8]
	pca9535_reg_output_0        // output bits [7:0] (default 1)
	pca9535_reg_output_1        // output [15:8]
	pca9535_reg_invert_polarity_0
	pca9535_reg_invert_polarity_1
	pca9535_reg_is_input_0 // 1 => pin is input; 0 => output
	pca9535_reg_is_input_1 // defaults are 1 (pin is input)
)

// MK1 pin usage.
const (
	mk1_pca9535_pin_switch_reset = 1 << iota
	_
	mk1_pca9535_pin_led_output_enable
)

// MK1 board front panel port LED's require PCA9535 GPIO device configuration
// to provide an output signal that allows LED operation.
func (m *pca9535_main) led_output_enable(bus *i2c.Bus) (err error) {
	var d i2c.SMBusData
	// Set pin to output (default is input and default value is high which we assume).
	if err = bus.Read(pca9535_reg_is_input_0, i2c.ByteData, &d); err != nil {
		return
	}
	d[0] &^= mk1_pca9535_pin_led_output_enable
	return bus.Write(pca9535_reg_is_input_0, i2c.ByteData, &d)
}

// Hard reset switch via gpio pins on MK1 board.
func (m *pca9535_main) switch_reset(bus *i2c.Bus) (err error) {
	const reset_bits = mk1_pca9535_pin_switch_reset

	//var val, dir i2c.SMBusData
	var val, out i2c.SMBusData

	// Set direction to output.
	if err = bus.Read(pca9535_reg_output_0, i2c.ByteData, &out); err != nil {
		return
	}
	if out[0]&reset_bits != 0 {
		out[0] &^= reset_bits
		if err = bus.Write(pca9535_reg_output_0, i2c.ByteData, &out); err != nil {
			return
		}
	}

	// Set output low & wait 2 us minimum.
	if err = bus.Read(pca9535_reg_is_input_0, i2c.ByteData, &val); err != nil {
		return
	}
	val[0] &^= reset_bits
	if err = bus.Write(pca9535_reg_is_input_0, i2c.ByteData, &val); err != nil {
		return
	}
	time.Sleep(2 * time.Microsecond)

	// Set output hi & wait 2 ms minimum before pci activity.
	val[0] |= reset_bits
	if err = bus.Write(pca9535_reg_is_input_0, i2c.ByteData, &val); err != nil {
		return
	}
	// Need to wait a long time else the switch does not show up in pci bus and pci discovery fails.
	time.Sleep(100 * time.Millisecond)

	return
}
