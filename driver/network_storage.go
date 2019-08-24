/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/danitso/terraform-provider-clouddk/clouddk"
)

const (
	nsFormatHostname      = "k8s-network-storage-%s"
	nsPathAPTAutoConf     = "/etc/apt/apt.conf.d/00auto-conf"
	nsPathBootstrapScript = "/tmp/clouddk_network_storage_bootstrap.sh"
	nsPathPublicKey       = "/root/.ssh/id_rsa_driver.pub"
)

var (
	nsAPTAutoConf = heredoc.Doc(`
		Dpkg::Options {
			"--force-confdef";
			"--force-confold";
		}
	`)
	nsBootstrapScript = heredoc.Doc(`
		#!/bin/bash
		set -e

		# Specify the required environment variables.
		export DEBIAN_FRONTEND=noninteractive

		# Authorize the SSH key and disable password authentication.
		if [[ ! -f /root/.ssh/authorized_keys ]]; then
			touch /root/.ssh/authorized_keys
		fi

		cat /root/.ssh/id_rsa_driver.pub >> /root/.ssh/authorized_keys
		sed -i 's/#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
		systemctl restart ssh

		# Turn off swap to improve performance.
		swapoff -a
		sed -i '/ swap / s/^/#/' /etc/fstab

		# Configure APT to use a mirror located in Denmark instead of the default US mirror.
		sed -i 's/us.archive.ubuntu.com/mirrors.dotsrc.org/' /etc/apt/sources.list

		# Wait for APT processes to terminate before proceeding.
		while ps aux | grep -q [a]pt || fuser /var/lib/apt/lists/lock >/dev/null 2>&1 || fuser /var/lib/dpkg/lock >/dev/null 2>&1; do
			sleep 2
		done

		# Upgrade the installed packages as the provided image is often quite old.
		apt-get -qq update
		apt-get -qq upgrade -y
		apt-get -qq dist-upgrade -y

		# Install some additional packages including the NFS server.
		apt-get -qq install -y \
			apt-transport-https \
			ca-certificates \
			nfs-kernel-server \
			software-properties-common
	`)
)

// NetworkStorage implements the logic for creating ReadWriteMany volumes.
type NetworkStorage struct {
	driver *Driver

	ID   string
	Size int
}

// createNetworkStorage creates new network storage of the given size.
func createNetworkStorage(d *Driver, name string, size int) (ns *NetworkStorage, err error) {
	hostname := fmt.Sprintf(nsFormatHostname, name)
	rootPassword := "p" + getRandomPassword(63)

	// Create a new storage server of the given size.
	debugCloudAction(rtNetworkStorage, "Creating server (hostname: %s)", hostname)

	body := clouddk.ServerCreateBody{
		Hostname:            hostname,
		Label:               hostname,
		InitialRootPassword: rootPassword,
		Package:             *d.PackageID,
		Template:            "ubuntu-18.04-x64",
		Location:            "dk1",
	}

	reqBody := new(bytes.Buffer)
	err = json.NewEncoder(reqBody).Encode(body)

	if err != nil {
		return nil, err
	}

	res, err := clouddk.DoClientRequest(d.Configuration.ClientSettings, "POST", "cloudservers", reqBody, []int{200}, 1, 1)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create server (hostname: %s)", hostname)

		return nil, err
	}

	server := clouddk.ServerBody{}
	err = json.NewDecoder(res.Body).Decode(&server)

	if err != nil {
		return nil, err
	}

	ns = &NetworkStorage{
		ID:   server.Identifier,
		Size: size,
	}

	// Wait for pending and running transactions to end.
	err = ns.Wait()

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to wait for pending and running transactions to end (id: %s)", ns.ID)

		ns.Delete()

		return nil, err
	}

	// Ensure that the server has at least a single network interface.
	debugCloudAction(rtNetworkStorage, "Checking network interfaces (id: %s)", ns.ID)

	if len(server.NetworkInterfaces) == 0 {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to lack of network interfaces (id: %s)", ns.ID)

		ns.Delete()

		return nil, fmt.Errorf("No network interfaces available (id: %s)", ns.ID)
	}

	// Create a data disk of the specified size.
	err = ns.EnsureDisk(size)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to disk creation error (id: %s)", ns.ID)
	}

	return nil, errors.New("Not implemented")
}

// loadNetworkStorage initializes the network storage handler for the given volume.
func loadNetworkStorage(d *Driver, id string) (ns *NetworkStorage, notFound bool, err error) {
	return nil, false, errors.New("Not implemented")
}

// Delete deletes the network storage.
func (ns *NetworkStorage) Delete() (err error) {
	debugCloudAction(rtNetworkStorage, "Deleting server (id: %s)", ns.ID)

	_, err = clouddk.DoClientRequest(
		ns.driver.Configuration.ClientSettings,
		"DELETE",
		fmt.Sprintf("cloudservers/%s", ns.ID),
		new(bytes.Buffer),
		[]int{200, 404},
		6,
		10,
	)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to delete server (id: %s)", ns.ID)

		return err
	}

	return nil
}

// EnsureDisk ensures that the server has a data disk of the specified size.
func (ns *NetworkStorage) EnsureDisk(size int) (err error) {
	debugCloudAction(rtNetworkStorage, "Ensuring disk (id: %s - size: %d GB)", ns.ID, size)

	return errors.New("Not implemented")
}

// Wait waits for any pending and running transactions to end.
func (ns *NetworkStorage) Wait() (err error) {
	timeDelay := int64(10)
	timeMax := float64(600)
	timeStart := time.Now()
	timeElapsed := timeStart.Sub(timeStart)

	wait := true

	for timeElapsed.Seconds() < timeMax {
		if int64(timeElapsed.Seconds())%timeDelay == 0 {
			res, err := clouddk.DoClientRequest(
				ns.driver.Configuration.ClientSettings,
				"GET",
				fmt.Sprintf("cloudservers/%s/logs", ns.ID),
				new(bytes.Buffer),
				[]int{200},
				1,
				1,
			)

			if err != nil {
				debugCloudAction(rtNetworkStorage, "Failed to retrieve logs (id: %s)", ns.ID)

				return err
			}

			logsList := clouddk.LogsListBody{}
			err = json.NewDecoder(res.Body).Decode(&logsList)

			if err != nil {
				return err
			}

			wait = false

			// Determine if there are any pending or running transactions.
			for _, v := range logsList {
				if v.Status == "pending" || v.Status == "running" {
					wait = true

					break
				}
			}

			if !wait {
				break
			}

			time.Sleep(1 * time.Second)
		}

		time.Sleep(200 * time.Millisecond)

		timeElapsed = time.Now().Sub(timeStart)
	}

	if wait {
		return errors.New("Timeout while waiting for transactions to end")
	}

	return nil
}
