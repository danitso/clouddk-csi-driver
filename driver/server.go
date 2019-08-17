/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"golang.org/x/crypto/ssh"
)

// CloudServer manages a Cloud.dk server.
type CloudServer struct {
	Driver      *Driver
	Information clouddk.ServerBody
}

// Create creates a new Cloud.dk server.
func (s *CloudServer) Create(locationID string, packageID string, hostname string) error {
	if s.Information.Identifier != "" {
		return errors.New("The server has already been initialized")
	}

	debugCloudAction(rtServers, "Creating cloud server (hostname: %s)", hostname)

	rootPassword := "p" + s.GetRandomPassword(63)

	body := clouddk.ServerCreateBody{
		Hostname:            hostname,
		Label:               hostname,
		InitialRootPassword: rootPassword,
		Package:             packageID,
		Template:            "ubuntu-18.04-x64",
		Location:            locationID,
	}

	reqBody := new(bytes.Buffer)
	err := json.NewEncoder(reqBody).Encode(body)

	if err != nil {
		return err
	}

	res, err := clouddk.DoClientRequest(s.Driver.Configuration.ClientSettings, "POST", "cloudservers", reqBody, []int{200}, 1, 1)

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server (hostname: %s)", hostname)

		return err
	}

	s.Information = clouddk.ServerBody{}
	err = json.NewDecoder(res.Body).Decode(&s.Information)

	if err != nil {
		return err
	}

	if len(s.Information.NetworkInterfaces) == 0 {
		debugCloudAction(rtServers, "Failed to create cloud server due to lack of network interfaces (hostname: %s)", hostname)

		err = fmt.Errorf("No network interfaces were created for cloud server '%s'", s.Information.Identifier)

		s.Destroy()

		return err
	}

	// Wait for the server to become ready by testing SSH connectivity.
	debugCloudAction(rtServers, "Waiting for cloud server to accept SSH connections (hostname: %s)", hostname)

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
			sshClient, err = ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

			if err == nil {
				break
			}

			time.Sleep(1 * time.Second)
		}

		time.Sleep(200 * time.Millisecond)

		timeElapsed = time.Now().Sub(timeStart)
	}

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to SSH timeout (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	defer sshClient.Close()

	s.Information.Booted = true

	// Configure the server by installing the required software and authorizing the SSH key.
	sshSession, err := sshClient.NewSession()

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to SSH errors (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	defer sshSession.Close()

	_, err = sshSession.CombinedOutput(
		"swapoff -a && " +
			"sed -i '/ swap / s/^/#/' /etc/fstab && " +
			fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys && ", strings.TrimSpace(s.Driver.Configuration.PublicKey)) +
			"sed -i 's/us.archive.ubuntu.com/mirrors.dotsrc.org/' /etc/apt/sources.list && " +
			"sed -i 's/#\\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config && " +
			"systemctl restart ssh",
	)

	if err != nil {
		debugCloudAction(rtServers, "Failed to create cloud server due to bootstrap errors (hostname: %s)", hostname)

		s.Destroy()

		return err
	}

	return nil
}

// Destroy destroys a Cloud.dk server.
func (s *CloudServer) Destroy() error {
	if s.Information.Identifier == "" {
		return errors.New("The server has not been initialized")
	}

	debugCloudAction(rtServers, "Destroying cloud server (hostname: %s)", s.Information.Hostname)

	_, err := clouddk.DoClientRequest(
		s.Driver.Configuration.ClientSettings,
		"DELETE",
		fmt.Sprintf("cloudservers/%s", s.Information.Identifier),
		new(bytes.Buffer),
		[]int{200, 404},
		60,
		10,
	)

	if err != nil {
		debugCloudAction(rtServers, "Failed to destroy cloud server (hostname: %s)", s.Information.Hostname)

		return err
	}

	s.Information = clouddk.ServerBody{}

	return nil
}

// GetRandomPassword generates a random password of a fixed length.
func (s *CloudServer) GetRandomPassword(length int) string {
	var b strings.Builder

	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}

	return b.String()
}

// InitializeByHostname initializes a CloudServer based on a hostname.
func (s *CloudServer) InitializeByHostname(hostname string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return false, errors.New("The server has already been initialized")
	}

	if hostname == "" {
		return false, errors.New("Cannot retrieve a server without a hostname")
	}

	res, err := clouddk.DoClientRequest(
		s.Driver.Configuration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers?hostname=%s", url.QueryEscape(hostname)),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		return false, err
	}

	servers := make(clouddk.ServerListBody, 0)
	err = json.NewDecoder(res.Body).Decode(&servers)

	if err != nil {
		return false, err
	}

	for _, v := range servers {
		if v.Hostname == hostname {
			s.Information = v

			return false, nil
		}
	}

	return true, fmt.Errorf("Failed to retrieve the server object for hostname '%s'", hostname)
}

// InitializeByID initializes a CloudServer based on an identifier.
func (s *CloudServer) InitializeByID(id string) (notFound bool, e error) {
	if s.Information.Identifier != "" {
		return false, errors.New("The server has already been initialized")
	}

	if id == "" {
		return false, errors.New("Cannot retrieve a server without an identifier")
	}

	res, err := clouddk.DoClientRequest(
		s.Driver.Configuration.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s", id),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		return (res.StatusCode == 404), err
	}

	err = json.NewDecoder(res.Body).Decode(&s.Information)

	if err != nil {
		return false, err
	}

	return false, nil
}

// SSH establishes a new SSH connection to a Cloud.dk server.
func (s *CloudServer) SSH() (*ssh.Client, error) {
	if s.Information.Identifier == "" {
		return nil, errors.New("The server has not been initialized")
	}

	sshPrivateKeyBuffer := bytes.NewBufferString(s.Driver.Configuration.PrivateKey)
	sshPrivateKeySigner, err := ssh.ParsePrivateKey(sshPrivateKeyBuffer.Bytes())

	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(sshPrivateKeySigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", s.Information.NetworkInterfaces[0].IPAddresses[0].Address+":22", sshConfig)

	if err != nil {
		return nil, err
	}

	return sshClient, nil
}
