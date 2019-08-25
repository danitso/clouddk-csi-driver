/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"context"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NodeServer implements the csi.NodeServer interface.
type NodeServer struct {
	driver *Driver
}

// newNodeServer creates a new node server.
func newNodeServer(d *Driver) *NodeServer {
	return &NodeServer{
		driver: d,
	}
}

// NodeExpandVolume expands the given volume.
func (ns *NodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeGetCapabilities returns the supported capabilities of the node server.
func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: ns.driver.NodeCapabilities,
	}, nil
}

// NodeGetInfo returns the supported capabilities of the node server.
// This is used so the CO knows where to place the workload.
// The result of this function will be used by the CO in ControllerPublishVolume.
func (ns *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.driver.Configuration.NodeID,
	}, nil
}

// NodeGetVolumeStats returns the volume capacity statistics available for the the given volume.
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume mounts the volume mounted to the staging path to the target path.
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	} else if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path must be provided")
	} else if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target Path must be provided")
	} else if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability must be provided")
	}

	// Separate the concatenated volume type and ID and attempt to revoke the node's access to the volume.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "Invalid volume ID")
	}

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		return nil, status.Error(codes.Unimplemented, "Block storage is not supported")
	case volumePrefixNetworkStorage:
		return nil, status.Error(codes.Unimplemented, "Work in progress")
	default:
		return nil, status.Error(codes.InvalidArgument, "Invalid volume type")
	}
}

// NodeStageVolume mounts the volume to a staging path on the node.
// This is called by the CO before NodePublishVolume and is used to temporary mount the volume to a staging path.
// Once mounted, NodePublishVolume will make sure to mount it to the appropriate path.
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	} else if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path must be provided")
	} else if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability must be provided")
	}

	// Separate the concatenated volume type and ID and attempt to revoke the node's access to the volume.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "Invalid volume ID")
	}

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		return nil, status.Error(codes.Unimplemented, "Block storage is not supported")
	case volumePrefixNetworkStorage:
		return nil, status.Error(codes.Unimplemented, "Work in progress")
	default:
		return nil, status.Error(codes.InvalidArgument, "Invalid volume type")
	}
}

// NodeUnpublishVolume unmounts the volume from the target path.
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	} else if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target Path must be provided")
	}

	// Separate the concatenated volume type and ID and attempt to revoke the node's access to the volume.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "Invalid volume ID")
	}

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		return nil, status.Error(codes.Unimplemented, "Block storage is not supported")
	case volumePrefixNetworkStorage:
		return nil, status.Error(codes.Unimplemented, "Work in progress")
	default:
		return nil, status.Error(codes.InvalidArgument, "Invalid volume type")
	}
}

// NodeUnstageVolume unstages the volume from the staging path.
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	} else if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging Target Path must be provided")
	}

	// Separate the concatenated volume type and ID and attempt to revoke the node's access to the volume.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "Invalid volume ID")
	}

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		return nil, status.Error(codes.Unimplemented, "Block storage is not supported")
	case volumePrefixNetworkStorage:
		return nil, status.Error(codes.Unimplemented, "Work in progress")
	default:
		return nil, status.Error(codes.InvalidArgument, "Invalid volume type")
	}
}
