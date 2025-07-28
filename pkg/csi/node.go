// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csi

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
)

func (*Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	d.volMutex.Lock()
	defer d.volMutex.Unlock()

	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if d.vol == nil {
		var err error
		if d.vol, err = createCacheVolume(ctx, d.client, d.nodeId, d.volumeTypeMap); err != nil {
			if errors.Is(err, &common.VolumePendingError{}) {
				return nil, status.Errorf(codes.Aborted, "local volume not ready: %v", err)
			}
			return nil, status.Error(codes.Internal, fmt.Sprintf("local volume creation failed: %v", err))
		}
	}

	targetPath := req.GetTargetPath()
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Errorf(codes.Internal, "Target mount point creation failed: %v", err)
			}
			notMnt = true
		} else {
			return nil, status.Errorf(codes.Internal, "Target mount point exists in bad state: %v", err)
		}
	}

	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	readOnly := req.GetReadonly()
	mount_options := []string{"bind"}
	if readOnly {
		mount_options = append(mount_options, "ro")
	}
	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      exec.New(),
	}
	if err := mounter.Interface.Mount(d.vol.Path(), targetPath, "", mount_options); err != nil {
		return nil, err
	}
	klog.Infof("Mounted %s to %s", d.vol.Path(), targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	d.volMutex.Lock()
	defer d.volMutex.Unlock()

	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      exec.New(),
	}
	err := mounter.Interface.Unmount(req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unmount of bind mount at %s failed: %v", req.GetTargetPath(), err)
	}

	klog.Infof("Unmounted %s", req.GetTargetPath())

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeId,
	}, nil
}
