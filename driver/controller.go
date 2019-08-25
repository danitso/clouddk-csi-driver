/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultVolumeCapacityInBytes = 17179869184
	maximumVolumeCapacityInBytes = 8796093022208
	minimumVolumeCapacityInBytes = 1073741824
	volumePrefixBlockStorage     = "bs"
	volumePrefixNetworkStorage   = "ns"
)

// ControllerServer implements the csi.ControllerServer interface.
type ControllerServer struct {
	driver *Driver
}

// newControllerServer creates a new identity server.
func newControllerServer(d *Driver) *ControllerServer {
	return &ControllerServer{
		driver: d,
	}
}

// ControllerGetCapabilities returns the capabilities of the controller service.
func (cs *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: cs.driver.ControllerCapabilities,
	}, nil
}

// ControllerExpandVolume expands the given volume.
func (cs *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume attaches the given volume to the node.
func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume deattaches the given volume from the node.
func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot will be called by the CO to create a new snapshot from a source volume on behalf of a user.
func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateVolume creates a new volume from the given request. The function is idempotent.
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: The volume name must be provided")
	} else if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: The volume capabilities must be provided")
	} else if req.VolumeContentSource != nil {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: Volume sources are not supported")
	}

	createNetworkStorage := false

	for _, cap := range req.VolumeCapabilities {
		supported := false

		for _, supportedCap := range cs.driver.VolumeCapabilities {
			if cap.AccessMode.Mode == supportedCap.AccessMode.Mode {
				supported = true

				switch cap.AccessMode.Mode {
				case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
					csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
					csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
					createNetworkStorage = true
				}

				break
			}
		}

		if !supported {
			return nil, status.Error(codes.InvalidArgument, "CreateVolume: Unsupported volume capabilities")
		}
	}

	capacityLimit := req.CapacityRange.GetLimitBytes()
	capacityLimitDefined := capacityLimit > 0
	capacityRequired := req.CapacityRange.GetRequiredBytes()
	capacityRequiredDefined := capacityRequired > 0

	// Determine if no capacity is specified in which case we can use the default volume capacity.
	if !capacityLimitDefined && !capacityRequiredDefined {
		capacityRequired = defaultVolumeCapacityInBytes
	}

	// Determine if the required capacity is less than the minimum supported capacity.
	if capacityRequiredDefined && capacityRequired < minimumVolumeCapacityInBytes {
		return nil, status.Error(codes.OutOfRange, "CreateVolume: The required capacity cannot be less than the minimum supported volume capacity")
	}

	// Determine if the capacity limit is less than the minimum supported capacity.
	if capacityLimitDefined && capacityLimit < minimumVolumeCapacityInBytes {
		return nil, status.Error(codes.OutOfRange, "CreateVolume: The capacity limit cannot be less than the minimum supported volume capacity")
	}

	// Determine if the required capacity is greater than the maximum supported capacity.
	if capacityRequiredDefined && capacityRequired > maximumVolumeCapacityInBytes {
		return nil, status.Error(codes.OutOfRange, "CreateVolume: The required capacity cannot be greater than the maximum supported volume capacity")
	}

	// Determine if the capacity limit is greater than the maximum supported capacity.
	if capacityLimitDefined && capacityLimit > maximumVolumeCapacityInBytes {
		return nil, status.Error(codes.OutOfRange, "CreateVolume: The capacity limit cannot be greater than the maximum supported volume capacity")
	}

	// Determine if the required capacity exceeds the capacity limit.
	if capacityRequiredDefined && capacityLimitDefined && capacityRequired > capacityLimit {
		return nil, status.Error(codes.OutOfRange, "CreateVolume: The required capacity is greater than the capacity limit")
	}

	// Create a new volume of the specified type.
	size := int(math.Ceil(math.Max(float64(capacityRequired), float64(capacityLimit)) / 1073741824))

	if createNetworkStorage {
		return cs.CreateVolumeNetworkStorage(ctx, req, size)
	}

	return cs.CreateVolumeBlockStorage(ctx, req, size)
}

// CreateVolumeBlockStorage creates new block storage from the given request. The function is idempotent.
func (cs *ControllerServer) CreateVolumeBlockStorage(ctx context.Context, req *csi.CreateVolumeRequest, size int) (*csi.CreateVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateVolume: Block storage has not been implemented")
}

// CreateVolumeNetworkStorage creates new network storage from the given request. The function is idempotent.
func (cs *ControllerServer) CreateVolumeNetworkStorage(ctx context.Context, req *csi.CreateVolumeRequest, size int) (*csi.CreateVolumeResponse, error) {
	ns, exists, err := createNetworkStorage(cs.driver, req.Name, size)

	if err != nil {
		if exists {
			return nil, status.Error(codes.AlreadyExists, "CreateVolume: The volume already exists")
		}

		return nil, status.Error(codes.Internal, "CreateVolume: "+err.Error())
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: int64(ns.Size * 1073741824),
			VolumeId:      fmt.Sprintf("%s-%s", volumePrefixNetworkStorage, ns.ID),
		},
	}, nil
}

// DeleteSnapshot will be called by the CO to delete a snapshot.
func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume: The volume ID must be provided")
	}

	// Separate the concatenated volume type and ID and attempt to delete the volume.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume: Invalid volume ID")
	}

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		return cs.DeleteVolumeBlockStorage(ctx, req, volumeInfo[1])
	case volumePrefixNetworkStorage:
		return cs.DeleteVolumeNetworkStorage(ctx, req, volumeInfo[1])
	default:
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume: Invalid volume type")
	}
}

// DeleteVolumeBlockStorage deletes the given block storage. The function is idempotent.
func (cs *ControllerServer) DeleteVolumeBlockStorage(ctx context.Context, req *csi.DeleteVolumeRequest, id string) (*csi.DeleteVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DeleteVolume: Block storage has not been implemented")
}

// DeleteVolumeNetworkStorage deletes the given network storage. The function is idempotent.
func (cs *ControllerServer) DeleteVolumeNetworkStorage(ctx context.Context, req *csi.DeleteVolumeRequest, id string) (*csi.DeleteVolumeResponse, error) {
	ns, notFound, err := loadNetworkStorage(cs.driver, id)

	if err != nil {
		if notFound {
			return &csi.DeleteVolumeResponse{}, nil
		}

		return nil, status.Error(codes.Internal, "DeleteVolume: "+err.Error())
	}

	err = ns.Delete()

	if err != nil {
		return nil, status.Error(codes.Internal, "DeleteVolume: "+err.Error())
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// GetCapacity returns the capacity of the storage pool.
func (cs *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots returns the information about all snapshots on the storage system within the given parameters regardless of how they were created.
// ListSnapshots shold not list a snapshot that is being created but has not been cut successfully yet.
func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes returns a list of all requested volumes.
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested are supported.
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities: The volume ID must be provided")
	} else if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities: The volume capabilities must be provided")
	}

	// Separate the concatenated volume type and ID.
	volumeInfo := strings.Split(req.VolumeId, "-")

	if len(volumeInfo) != 2 {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities: Invalid volume ID")
	}

	// Determine the volume capabilities based on the volume type.
	var supportedCaps []*csi.VolumeCapability

	switch volumeInfo[0] {
	case volumePrefixBlockStorage:
		supportedCaps = []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		}
	case volumePrefixNetworkStorage:
		_, notFound, err := loadNetworkStorage(cs.driver, volumeInfo[1])

		if err != nil {
			if notFound {
				return nil, status.Error(codes.NotFound, "ValidateVolumeCapabilities: The specified volume does not exist")
			}

			return nil, status.Error(codes.Internal, "ValidateVolumeCapabilities: "+err.Error())
		}

		supportedCaps = []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities: Invalid volume type")
	}

	// Verify that the requested volume capabilities match the supported capabilities.
	confirmedCaps := []*csi.VolumeCapability{}

	for _, cap := range req.VolumeCapabilities {
		for _, supportedCap := range supportedCaps {
			if cap.AccessMode.Mode == supportedCap.AccessMode.Mode {
				confirmedCaps = append(confirmedCaps, cap)

				break
			}
		}
	}

	if len(confirmedCaps) != len(req.VolumeCapabilities) {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities: Unsupported volume capabilities")
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: confirmedCaps,
		},
	}, nil
}
