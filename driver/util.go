/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"fmt"
	"log"
	"strings"
)

const (
	rtServers = "SERVERS"
	rtVolumes = "VOLUMES"
)

// debugCloudAction writes a debug message to the log.
func debugCloudAction(resourceType string, format string, v ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] ", resourceType)+format, v...)
}

// trimProviderID removes the provider name from the id.
func trimProviderID(id string) string {
	return strings.TrimPrefix(id, "clouddk://")
}
