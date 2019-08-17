/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/danitso/clouddk-csi-driver/driver"
	"github.com/danitso/terraform-provider-clouddk/clouddk"
)

const (
	// envAPIEndpoint specifies the name of the environment variable containing the Cloud.dk API endpoint.
	envAPIEndpoint = "CLOUDDK_API_ENDPOINT"

	// envAPIKey specifies the name of the environment variable containing the Cloud.dk API key.
	envAPIKey = "CLOUDDK_API_KEY"

	// envCSIEndpointKey specifies the name of the environment variable containing the CSI endpoint.
	envCSIEndpointKey = "CLOUDDK_CSI_ENDPOINT"

	// envNodeID specifies the name of the environment variable containing the node identifier.
	envNodeID = "CLOUDDK_NODE_ID"

	// envServerMemory specifies the name of the environment variable containing the amount of memory per storage server.
	envServerMemory = "CLOUDDK_SERVER_MEMORY"

	// envServerProcessors specifies the name of the environment variable containing the number of processors per storage server.
	envServerProcessors = "CLOUDDK_SERVER_PROCESSORS"

	// envSSHPrivateKey specifies the name of the environment variable containing the Base64 encoded private key for SSH connections.
	envSSHPrivateKey = "CLOUDDK_SSH_PRIVATE_KEY"

	// envSSHPublicKey specifies the name of the environment variable containing the Base64 encoded public key for SSH connections.
	envSSHPublicKey = "CLOUDDK_SSH_PUBLIC_KEY"

	// flagAPIEndpoint specifies the name of the command line option containing the Cloud.dk API endpoint.
	flagAPIEndpoint = "api-endpoint"

	// flagAPIKey specifies the name of the command line option containing the Cloud.dk API key.
	flagAPIKey = "api-key"

	// flagCSIEndpoint specifies the name of the command line option containing the CSI endpoint.
	flagCSIEndpoint = "csi-endpoint"

	// flagNodeID specifies the name of the command line option containing the node identifier.
	flagNodeID = "node-id"

	// flagServerMemory specifies the name of the command line option containing the amount of memory per storage server.
	flagServerMemory = "server-memory"

	// flagServerProcessors specifies the name of the command line option containing the number of processors per storage server.
	flagServerProcessors = "server-processors"

	// flagSSHPrivateKey specifies the name of the command line option containing the Base64 encoded private key for SSH connections.
	flagSSHPrivateKey = "ssh-private-key"

	// flagSSHPublicKey specifies the name of the command line option containing the Base64 encoded public key for SSH connections.
	flagSSHPublicKey = "ssh-public-key"
)

func main() {
	// Parse the environment variables and command line flags.
	var (
		apiEndpointEnv      = os.Getenv(envAPIEndpoint)
		apiKeyEnv           = os.Getenv(envAPIKey)
		csiEndpointEnv      = os.Getenv(envCSIEndpointKey)
		nodeIDEnv           = os.Getenv(envNodeID)
		serverMemoryEnv     = os.Getenv(envServerMemory)
		serverProcessorsEnv = os.Getenv(envServerProcessors)
		sshPrivateKeyEnv    = os.Getenv(envSSHPrivateKey)
		sshPublicKeyEnv     = os.Getenv(envSSHPublicKey)
	)

	if apiEndpointEnv == "" {
		apiEndpointEnv = "https://api.cloud.dk/v1"
	}

	if csiEndpointEnv == "" {
		csiEndpointEnv = "unix:///var/lib/kubelet/plugins/" + driver.DriverName + "/csi.sock"
	}

	serverMemory := 4096
	serverProcessors := 2

	if serverMemoryEnv != "" {
		i, err := strconv.Atoi(serverMemoryEnv)

		if err != nil {
			log.Fatalln(err)
		}

		serverMemory = i
	}

	if serverProcessorsEnv != "" {
		i, err := strconv.Atoi(serverProcessorsEnv)

		if err != nil {
			log.Fatalln(err)
		}

		serverProcessors = i
	}

	var (
		apiEndpointFlag      = flag.String(flagAPIEndpoint, apiEndpointEnv, "The API endpoint")
		apiKeyFlag           = flag.String(flagAPIKey, apiKeyEnv, "The API key")
		csiEndpointFlag      = flag.String(flagCSIEndpoint, csiEndpointEnv, "The CSI endpoint")
		nodeIDFlag           = flag.String(flagNodeID, nodeIDEnv, "The node id")
		serverMemoryFlag     = flag.Int(flagServerMemory, serverMemory, "The minimum amount of memory per storage server")
		serverProcessorsFlag = flag.Int(flagServerProcessors, serverProcessors, "The minimum number of processors per storage server")
		sshPrivateKeyFlag    = flag.String(flagSSHPrivateKey, sshPrivateKeyEnv, "The Base64 encoded private key for SSH connections")
		sshPublicKeyFlag     = flag.String(flagSSHPublicKey, sshPublicKeyEnv, "The Base64 encoded public key for SSH connections")
	)

	flag.Parse()

	// Decode the private and public SSH keys.
	if *sshPrivateKeyFlag != "" {
		key, err := base64.StdEncoding.DecodeString(*sshPrivateKeyFlag)

		if err != nil {
			log.Fatalln(err)
		}

		*sshPrivateKeyFlag = bytes.NewBuffer(key).String()
	}

	if *sshPublicKeyFlag != "" {
		key, err := base64.StdEncoding.DecodeString(*sshPublicKeyFlag)

		if err != nil {
			log.Fatalln(err)
		}

		*sshPublicKeyFlag = bytes.NewBuffer(key).String()
	}

	// Initialize the driver.
	c := driver.Configuration{
		ClientSettings: &clouddk.ClientSettings{
			Endpoint: *apiEndpointFlag,
			Key:      *apiKeyFlag,
		},
		Endpoint:         *csiEndpointFlag,
		NodeID:           *nodeIDFlag,
		PrivateKey:       *sshPrivateKeyFlag,
		PublicKey:        *sshPublicKeyFlag,
		ServerMemory:     *serverMemoryFlag,
		ServerProcessors: *serverProcessorsFlag,
	}

	drv, err := driver.NewDriver(&c)

	if err != nil {
		log.Fatalln(err)
	}

	drv.Run()
}
