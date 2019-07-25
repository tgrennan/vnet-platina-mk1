// autogenerated: do not edit!
// generated from gentemplate [gentemplate -d Package=main -id ifStatsPollerInterface -d VecType=ifStatsPollerInterfaceVec -d Type=ifStatsPollerInterface github.com/platinasystems/elib/vec.tmpl]

// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"github.com/platinasystems/elib"
)

type ifStatsPollerInterfaceVec []ifStatsPollerInterface

func (p *ifStatsPollerInterfaceVec) Resize(n uint) {
	c := uint(cap(*p))
	l := uint(len(*p)) + n
	if l > c {
		c = elib.NextResizeCap(l)
		q := make([]ifStatsPollerInterface, l, c)
		copy(q, *p)
		*p = q
	}
	*p = (*p)[:l]
}

func (p *ifStatsPollerInterfaceVec) validate(new_len uint, zero ifStatsPollerInterface) *ifStatsPollerInterface {
	c := uint(cap(*p))
	lʹ := uint(len(*p))
	l := new_len
	if l <= c {
		// Need to reslice to larger length?
		if l > lʹ {
			*p = (*p)[:l]
			for i := lʹ; i < l; i++ {
				(*p)[i] = zero
			}
		}
		return &(*p)[l-1]
	}
	return p.validateSlowPath(zero, c, l, lʹ)
}

func (p *ifStatsPollerInterfaceVec) validateSlowPath(zero ifStatsPollerInterface, c, l, lʹ uint) *ifStatsPollerInterface {
	if l > c {
		cNext := elib.NextResizeCap(l)
		q := make([]ifStatsPollerInterface, cNext, cNext)
		copy(q, *p)
		for i := c; i < cNext; i++ {
			q[i] = zero
		}
		*p = q[:l]
	}
	if l > lʹ {
		*p = (*p)[:l]
	}
	return &(*p)[l-1]
}

func (p *ifStatsPollerInterfaceVec) Validate(i uint) *ifStatsPollerInterface {
	var zero ifStatsPollerInterface
	return p.validate(i+1, zero)
}

func (p *ifStatsPollerInterfaceVec) ValidateInit(i uint, zero ifStatsPollerInterface) *ifStatsPollerInterface {
	return p.validate(i+1, zero)
}

func (p *ifStatsPollerInterfaceVec) ValidateLen(l uint) (v *ifStatsPollerInterface) {
	if l > 0 {
		var zero ifStatsPollerInterface
		v = p.validate(l, zero)
	}
	return
}

func (p *ifStatsPollerInterfaceVec) ValidateLenInit(l uint, zero ifStatsPollerInterface) (v *ifStatsPollerInterface) {
	if l > 0 {
		v = p.validate(l, zero)
	}
	return
}

func (p *ifStatsPollerInterfaceVec) ResetLen() {
	if *p != nil {
		*p = (*p)[:0]
	}
}

func (p ifStatsPollerInterfaceVec) Len() uint { return uint(len(p)) }
