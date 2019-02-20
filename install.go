// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func install() error {
	const dn = "/usr/lib/goes"
	if os.Geteuid() != 0 {
		return errors.New("you aren't root")
	}
	self, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}
	r, err := os.Open(self)
	if err != nil {
		return err
	}
	defer r.Close()
	fn := filepath.Join(dn, filepath.Base(self))
	di, err := os.Stat(dn)
	if os.IsNotExist(err) {
		if err = os.Mkdir(dn, 0755); err != nil {
			return err
		}
	} else if !di.IsDir() {
		return fmt.Errorf("%s isn't a directory", dn)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	mode := os.FileMode(0755)
	w, err := os.OpenFile(fn, flags, mode)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return err

}
