// Copyright © 2016-2018 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import "strings"

// translated counter name
func xCounter(s string) string {
	s = counterSeparators().Replace(s)
	if x, found := linkStatTranslation[s]; found {
		s = x
	}
	return s
}

var cachedCounterSeparators *strings.Replacer

func counterSeparators() *strings.Replacer {
	if cachedCounterSeparators == nil {
		cachedCounterSeparators =
			strings.NewReplacer(" ", "-", ".", "-", "_", "-")
	}
	return cachedCounterSeparators
}

var linkStatTranslation = map[string]string{
	"port-rx-multicast-packets":     "multicast",
	"port-rx-bytes":                 "rx-bytes",
	"port-rx-crc_error-packets":     "rx-crc-errors",
	"port-rx-runt-packets":          "rx-fifo-errors",
	"port-rx-undersize-packets":     "rx-length-errors",
	"port-rx-oversize-packets":      "rx-over-errors",
	"port-rx-packets":               "rx-packets",
	"port-tx-total-collisions":      "collisions",
	"port-tx-fifo-underrun-packets": "tx-aborted-errors",
	"port-tx-bytes":                 "tx-bytes",
	"port-tx-runt-packets":          "tx-fifo-errors",
	"port-tx-packets":               "tx-packets",
}
