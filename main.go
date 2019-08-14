/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"

	"github.com/danitso/clouddk-csi-driver/driver"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/"+driver.DriverName+"/csi.sock", "CSI endpoint")
		nodeID   = flag.String("nodeid", "", "node id")
	)

	flag.Parse()

	drv, err := driver.NewDriver(*nodeID, *endpoint)

	if err != nil {
		log.Fatalln(err)
	}

	drv.Run()
}
