/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
)

const (
	// DriverName defines the name that is used in Kubernetes and the CSI system for the canonical, official name of this plugin.
	DriverName = "csi.cloud.dk"

	// DriverVersion defines the driver's version number.
	DriverVersion = "0.1.0"
)

// Driver exposes the CSI driver for Cloud.dk.
type Driver struct {
	driver *csicommon.CSIDriver

	endpoint string
	nodeID   string

	controllerServer *ControllerServer
	identityServer   *IdentityServer
	nodeServer       *NodeServer

	controllerCapabilities []*csi.ControllerServiceCapability
	nodeCapabilities       []*csi.NodeServiceCapability
	volumeCapabilities     []*csi.VolumeCapability_AccessMode
}

// NewDriver returns a CSI plugin that manages Cloud.dk block storage
func NewDriver(nodeID, endpoint string) (*Driver, error) {
	return &Driver{
		endpoint: endpoint,
		nodeID:   nodeID,
		controllerCapabilities: []*csi.ControllerServiceCapability{
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
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
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
		nodeCapabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
		volumeCapabilities: []*csi.VolumeCapability_AccessMode{
			{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
		},
	}, nil
}

// Run starts the CSI driver.
func (d *Driver) Run() {
	log.Printf("Running CSI driver '%s' version %s", DriverName, DriverVersion)

	d.driver = csicommon.NewCSIDriver(DriverName, DriverVersion, d.nodeID)

	if d.driver == nil {
		log.Fatalf("Failed to initialize CSI Driver '%s'", DriverName)
	}

	csCaps := []csi.ControllerServiceCapability_RPC_Type{}

	for _, cap := range d.controllerCapabilities {
		csCaps = append(csCaps, cap.Type.(*csi.ControllerServiceCapability_Rpc).Rpc.Type)
	}

	volCaps := []csi.VolumeCapability_AccessMode_Mode{}

	for _, cap := range d.volumeCapabilities {
		volCaps = append(volCaps, cap.Mode)
	}

	d.driver.AddControllerServiceCapabilities(csCaps)
	d.driver.AddVolumeCapabilityAccessModes(volCaps)

	d.controllerServer = newControllerServer(d)
	d.identityServer = newIdentityServer(d)
	d.nodeServer = newNodeServer(d)

	s := csicommon.NewNonBlockingGRPCServer()
	s.Start(d.endpoint, d.identityServer, d.controllerServer, d.nodeServer)
	s.Wait()
}
