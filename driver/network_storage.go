/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"errors"
)

// NetworkStorage implements the logic for creating ReadWriteMany volumes.
type NetworkStorage struct {
	driver *Driver

	ID   string
	Size int
}

// createNetworkStorage creates new network storage of the given size.
func createNetworkStorage(d *Driver, size int) (ns *NetworkStorage, err error) {
	ns = &NetworkStorage{
		driver: d,
		Size:   size,
	}

	return ns, errors.New("Not implemented")
}

// loadNetworkStorage initializes the network storage handler for the given volume.
func loadNetworkStorage(d *Driver, id string) (ns *NetworkStorage, notFound bool, err error) {
	ns = &NetworkStorage{
		driver: d,
		ID:     id,
	}

	return ns, false, errors.New("Not implemented")
}

// Delete deletes the network storage.
func (ns *NetworkStorage) Delete() (notFound bool, err error) {
	return false, errors.New("Not implemented")
}
