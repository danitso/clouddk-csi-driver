/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/danitso/terraform-provider-clouddk/clouddk"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
)

const (
	// DriverName defines the name that is used in Kubernetes and the CSI system for the canonical, official name of this plugin.
	DriverName = "csi.cloud.dk"

	// DriverVersion defines the driver's version number.
	DriverVersion = "0.1.0"
)

// Configuration stores the driver configuration.
type Configuration struct {
	ClientSettings   *clouddk.ClientSettings
	Endpoint         string
	NodeID           string
	PrivateKey       string
	PublicKey        string
	ServerMemory     int
	ServerProcessors int
}

// Driver exposes the CSI driver for Cloud.dk.
type Driver struct {
	Configuration *Configuration
	Driver        *csicommon.CSIDriver
	PackageID     *string

	ControllerServer *ControllerServer
	IdentityServer   *IdentityServer
	NodeServer       *NodeServer

	ControllerCapabilities []*csi.ControllerServiceCapability
	NodeCapabilities       []*csi.NodeServiceCapability
	PluginCapabilities     []*csi.PluginCapability
	VolumeCapabilities     []*csi.VolumeCapability
}

// NewDriver returns a CSI plugin that manages Cloud.dk block storage
func NewDriver(c *Configuration) (*Driver, error) {
	packageID, err := getPackageID(c.ServerMemory, c.ServerProcessors)

	if err != nil {
		return nil, err
	}

	return &Driver{
		Configuration: c,
		PackageID:     packageID,
		ControllerCapabilities: []*csi.ControllerServiceCapability{
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
		},
		NodeCapabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
		PluginCapabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		},
	}, nil
}

// Run starts the CSI driver.
func (d *Driver) Run() {
	log.Printf("Running CSI driver '%s' version %s", DriverName, DriverVersion)

	d.Driver = csicommon.NewCSIDriver(DriverName, DriverVersion, d.Configuration.NodeID)

	if d.Driver == nil {
		log.Fatalf("Failed to initialize CSI Driver '%s'", DriverName)
	}

	csCaps := []csi.ControllerServiceCapability_RPC_Type{}

	for _, cap := range d.ControllerCapabilities {
		csCaps = append(csCaps, cap.Type.(*csi.ControllerServiceCapability_Rpc).Rpc.Type)
	}

	volCaps := []csi.VolumeCapability_AccessMode_Mode{}

	for _, cap := range d.VolumeCapabilities {
		volCaps = append(volCaps, cap.AccessMode.Mode)
	}

	d.Driver.AddControllerServiceCapabilities(csCaps)
	d.Driver.AddVolumeCapabilityAccessModes(volCaps)

	d.ControllerServer = newControllerServer(d)
	d.IdentityServer = newIdentityServer(d)
	d.NodeServer = newNodeServer(d)

	s := csicommon.NewNonBlockingGRPCServer()
	s.Start(d.Configuration.Endpoint, d.IdentityServer, d.ControllerServer, d.NodeServer)
	s.Wait()
}
