/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	nsDiskLabel           = "k8s-network-storage"
	nsFormatHostname      = "k8s-network-storage-%s"
	nsPathAPTAutoConf     = "/etc/apt/apt.conf.d/00auto-conf"
	nsPathBootstrapScript = "/etc/clouddk_network_storage_bootstrap.sh"
	nsPathMountScript     = "/etc/clouddk_network_storage_mount.sh"
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

		# Install some additional packages including the NFS kernel server.
		apt-get -qq install -y \
			apt-transport-https \
			ca-certificates \
			nfs-kernel-server \
			software-properties-common
	`)
	nsMountScript = heredoc.Doc(`
		#!/bin/bash
		set -e

		# Specify the device and directory.
		DATA_DEVICE="/dev/vdb"
		DATA_DIRECTORY="/mnt/data"

		# Ensure that the device is mounted.
		if ! mountpoint -q "$DATA_DIRECTORY"; then
			if [[ "$(blkid -s TYPE -o value "$DATA_DEVICE")" == "" ]]; then
				mkfs -t ext4 "$DATA_DEVICE"
			fi

			if ! grep -q "$DATA_DIRECTORY" /etc/fstab; then
				data_device_uuid="$(blkid -s UUID -o value "$DATA_DEVICE")"

				sed --in-place "/${DATA_DEVICE//'/'/'\/'}/d" /etc/fstab
				echo "UUID=${data_device_uuid} ${DATA_DIRECTORY} ext4 defaults,noatime,nodiratime,nofail 0 2" >> /etc/fstab
			fi

			mkdir -p "$DATA_DIRECTORY"
			mount "$DATA_DEVICE" "$DATA_DIRECTORY"
			chown -R nobody:nogroup "$DATA_DIRECTORY"
		fi
	`)
)

// NetworkStorage implements the logic for creating ReadWriteMany volumes.
type NetworkStorage struct {
	driver *Driver

	ID   string
	IP   string
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

	// Ensure that the server has at least a single network interface.
	debugCloudAction(rtNetworkStorage, "Checking network interfaces (id: %s)", ns.ID)

	if len(server.NetworkInterfaces) == 0 {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to lack of network interfaces (id: %s)", ns.ID)

		ns.Delete()

		return nil, fmt.Errorf("No network interfaces available (id: %s)", ns.ID)
	}

	ns.IP = server.NetworkInterfaces[0].IPAddresses[0].Address

	// Wait for pending and running transactions to end.
	err = ns.Wait()

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to active transactions (id: %s)", ns.ID)

		ns.Delete()

		return nil, err
	}

	// Wait for the server to become ready by testing SSH connectivity.
	debugCloudAction(rtNetworkStorage, "Waiting for server to accept SSH connections (id: %s)", ns.ID)

	var sshClient *ssh.Client

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password(rootPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	timeDelay := int64(10)
	timeMax := float64(300)
	timeStart := time.Now()
	timeElapsed := timeStart.Sub(timeStart)

	err = nil

	for timeElapsed.Seconds() < timeMax {
		if int64(timeElapsed.Seconds())%timeDelay == 0 {
			sshClient, err = ssh.Dial("tcp", ns.IP+":22", sshConfig)

			if err == nil {
				break
			}

			time.Sleep(1 * time.Second)
		}

		time.Sleep(200 * time.Millisecond)

		timeElapsed = time.Now().Sub(timeStart)
	}

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create server due to SSH timeout (id: %s)", ns.ID)

		ns.Delete()

		return nil, err
	}

	defer sshClient.Close()

	// Create a new SFTP client.
	sftpClient, err := ns.CreateSFTPClient(sshClient)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to SFTP errors (id: %s)", ns.ID)

		ns.Delete()

		return nil, err
	}

	defer sftpClient.Close()

	// Upload files and scripts to the server.
	err = ns.CreateFile(sftpClient, nsPathAPTAutoConf, bytes.NewBufferString(strings.ReplaceAll(nsAPTAutoConf, "\r", "")))

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server because file '%s' could not be created (id: %s)", nsPathAPTAutoConf, ns.ID)

		ns.Delete()

		return nil, err
	}

	err = ns.CreateFile(sftpClient, nsPathBootstrapScript, bytes.NewBufferString(strings.ReplaceAll(nsBootstrapScript, "\r", "")))

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server because file '%s' could not be created (id: %s)", nsPathBootstrapScript, ns.ID)

		ns.Delete()

		return nil, err
	}

	err = ns.CreateFile(sftpClient, nsPathMountScript, bytes.NewBufferString(strings.ReplaceAll(nsMountScript, "\r", "")))

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server because file '%s' could not be created (id: %s)", nsPathMountScript, ns.ID)

		ns.Delete()

		return nil, err
	}

	err = ns.CreateFile(sftpClient, nsPathPublicKey, bytes.NewBufferString(strings.ReplaceAll(ns.driver.Configuration.PublicKey, "\r", "")))

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server because file '%s' could not be created (id: %s)", nsPathPublicKey, ns.ID)

		ns.Delete()

		return nil, err
	}

	// Create a new SSH session and execute the bootstrap script.
	sshSession, err := ns.CreateSSHSession(sshClient)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to initialize server due to SSH session errors (id: %s)", ns.ID)

		ns.Delete()

		return nil, err
	}

	defer sshSession.Close()

	debugCloudAction(rtNetworkStorage, "Bootstrapping server (id: %s)", ns.ID)

	output, err := sshSession.CombinedOutput("/bin/bash " + nsPathBootstrapScript)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to bootstrap server (id: %s) - Output: %s - Error: %s", ns.ID, string(output), err.Error())

		ns.Delete()

		return nil, err
	}

	// Create the data disk.
	err = ns.EnsureDisk(size)

	if err != nil {
		ns.Delete()

		return nil, err
	}

	return ns, nil
}

// loadNetworkStorage initializes the network storage handler for the given volume.
func loadNetworkStorage(d *Driver, id string) (ns *NetworkStorage, notFound bool, err error) {
	res, err := clouddk.DoClientRequest(
		d.Configuration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s", id),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to load server (id: %s)", id)

		return nil, (res.StatusCode == 404), err
	}

	server := clouddk.ServerBody{}
	err = json.NewDecoder(res.Body).Decode(&server)

	if err != nil {
		return nil, false, err
	}

	if len(server.NetworkInterfaces) == 0 {
		debugCloudAction(rtNetworkStorage, "Failed to load server due to lack of network interfaces (id: %s)", id)

		return nil, false, fmt.Errorf("The server has no network interfaces (id: %s)", id)
	}

	ns = &NetworkStorage{
		ID: server.Identifier,
		IP: server.NetworkInterfaces[0].IPAddresses[0].Address,
	}

	for _, v := range server.Disks {
		if v.Label == nsDiskLabel {
			ns.Size = int(v.Size)

			break
		}
	}

	return ns, false, nil
}

// CreateFile creates a file on the server.
func (ns *NetworkStorage) CreateFile(sftpClient *sftp.Client, filePath string, fileContents *bytes.Buffer) error {
	debugCloudAction(rtNetworkStorage, "Creating file '%s' (id: %s)", filePath, ns.ID)

	newSFTPClient := sftpClient

	if newSFTPClient == nil {
		sshClient, err := ns.CreateSSHClient()

		if err != nil {
			return err
		}

		defer sshClient.Close()

		newSFTPClient, err = ns.CreateSFTPClient(sshClient)

		if err != nil {
			return err
		}

		defer newSFTPClient.Close()
	}

	dir := filepath.Dir(filePath)
	err := newSFTPClient.MkdirAll(dir)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create directory '%s' (id: %s)", dir, ns.ID)

		return err
	}

	remoteFile, err := newSFTPClient.Create(filePath)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create file '%s' (id: %s)", filePath, ns.ID)

		return err
	}

	defer remoteFile.Close()

	_, err = remoteFile.ReadFrom(fileContents)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to write file '%s' (id: %s)", filePath, ns.ID)

		return err
	}

	return nil
}

// CreateSFTPClient creates an SFTP client.
func (ns *NetworkStorage) CreateSFTPClient(sshClient *ssh.Client) (*sftp.Client, error) {
	debugCloudAction(rtNetworkStorage, "Creating SFTP client (id: %s)", ns.ID)

	var err error

	newSSHClient := sshClient

	if newSSHClient == nil {
		newSSHClient, err = ns.CreateSSHClient()

		if err != nil {
			debugCloudAction(rtNetworkStorage, "Failed to create SFTP client due to SSH errors (id: %s)", ns.ID)

			return nil, err
		}
	}

	sftpClient, err := sftp.NewClient(newSSHClient)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create SFTP client (id: %s)", ns.ID)

		return nil, err
	}

	return sftpClient, nil
}

// CreateSSHClient establishes a new SSH connection to the server.
func (ns *NetworkStorage) CreateSSHClient() (*ssh.Client, error) {
	debugCloudAction(rtNetworkStorage, "Creating SSH client (id: %s)", ns.ID)

	sshPrivateKeyBuffer := bytes.NewBufferString(ns.driver.Configuration.PrivateKey)
	sshPrivateKeySigner, err := ssh.ParsePrivateKey(sshPrivateKeyBuffer.Bytes())

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create SSH client due to private key errors (id: %s)", ns.ID)

		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(sshPrivateKeySigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", ns.IP+":22", sshConfig)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create SSH client (id: %s)", ns.ID)

		return nil, err
	}

	return sshClient, nil
}

// CreateSSHSession creates an SSH session.
func (ns *NetworkStorage) CreateSSHSession(sshClient *ssh.Client) (*ssh.Session, error) {
	debugCloudAction(rtNetworkStorage, "Creating SSH session (id: %s)", ns.ID)

	var err error

	newSSHClient := sshClient

	if newSSHClient == nil {
		newSSHClient, err = ns.CreateSSHClient()

		if err != nil {
			debugCloudAction(rtNetworkStorage, "Failed to create SSH session due to SSH errors (id: %s)", ns.ID)

			return nil, err
		}
	}

	sshSession, err := newSSHClient.NewSession()

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to create SSH session (id: %s)", ns.ID)

		return nil, err
	}

	return sshSession, nil
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

	// Wait for all transactions to end before proceeding.
	err = ns.Wait()

	if err != nil {
		return err
	}

	// Retrieve the list of disks attached to the server and determine if a data disk is present.
	res, err := clouddk.DoClientRequest(
		ns.driver.Configuration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s/disks", ns.ID),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to retrieve list of disks (id: %s)", ns.ID)

		return err
	}

	diskList := clouddk.DiskListBody{}
	err = json.NewDecoder(res.Body).Decode(&diskList)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to decode list of disks (id: %s)", ns.ID)

		return err
	}

	diskFound := false

	for _, v := range diskList {
		if v.Label == nsDiskLabel {
			diskFound = true

			break
		}
	}

	// Create a new data disk and wait for it to become attached.
	if !diskFound {
		debugCloudAction(rtNetworkStorage, "Creating data disk (id: %s - size: %d GB)", ns.ID, size)

		createBody := clouddk.DiskCreateBody{
			Label: nsDiskLabel,
			Size:  clouddk.CustomInt(size),
		}

		reqBody := new(bytes.Buffer)
		err = json.NewEncoder(reqBody).Encode(createBody)

		if err != nil {
			return err
		}

		res, err = clouddk.DoClientRequest(
			ns.driver.Configuration.ClientSettings,
			"POST",
			fmt.Sprintf("cloudservers/%s/disks", ns.ID),
			reqBody,
			[]int{200},
			1,
			1,
		)

		if err != nil {
			debugCloudAction(rtNetworkStorage, "Failed to create data disk (id: %s)", ns.ID)

			return err
		}

		disk := clouddk.DiskBody{}
		err = json.NewDecoder(res.Body).Decode(&disk)

		if err != nil {
			return err
		}

		err = ns.Wait()

		if err != nil {
			return err
		}
	}

	// Mount the data disk, if necessary.
	sshSession, err := ns.CreateSSHSession(nil)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to ensure disk due to SSH session errors (id: %s)", ns.ID)

		return err
	}

	defer sshSession.Close()

	debugCloudAction(rtNetworkStorage, "Mounting data disk (id: %s)", ns.ID)

	output, err := sshSession.CombinedOutput("/bin/bash " + nsPathMountScript)

	if err != nil {
		debugCloudAction(rtNetworkStorage, "Failed to mount data disk (id: %s) - Output: %s - Error: %s", ns.ID, string(output), err.Error())

		return err
	}

	return nil
}

// Wait waits for any pending and running transactions to end.
func (ns *NetworkStorage) Wait() (err error) {
	debugCloudAction(rtNetworkStorage, "Waiting for transactions to end (id: %s)", ns.ID)

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
				debugCloudAction(rtNetworkStorage, "Failed to retrieve list of transactions (id: %s)", ns.ID)

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
		debugCloudAction(rtNetworkStorage, "Timeout while waiting for transactions to end (id: %s)", ns.ID)

		return errors.New("Timeout while waiting for transactions to end")
	}

	return nil
}