package server

import (
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DevnetToProto converts a domain Devnet to a proto Devnet.
func DevnetToProto(d *types.Devnet) *v1.Devnet {
	if d == nil {
		return nil
	}

	return &v1.Devnet{
		Metadata: metadataToProto(&d.Metadata),
		Spec:     specToProto(&d.Spec),
		Status:   statusToProto(&d.Status),
	}
}

// DevnetFromProto converts a proto Devnet to a domain Devnet.
func DevnetFromProto(pb *v1.Devnet) *types.Devnet {
	if pb == nil {
		return nil
	}

	return &types.Devnet{
		Metadata: metadataFromProto(pb.Metadata),
		Spec:     specFromProto(pb.Spec),
		Status:   statusFromProto(pb.Status),
	}
}

// CreateRequestToDevnet converts a CreateDevnetRequest to a domain Devnet.
func CreateRequestToDevnet(req *v1.CreateDevnetRequest) *types.Devnet {
	now := time.Now()

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:       req.Name,
			Generation: 1,
			CreatedAt:  now,
			UpdatedAt:  now,
			Labels:     req.Labels,
		},
		Status: types.DevnetStatus{
			Phase: types.PhasePending,
		},
	}

	if req.Spec != nil {
		devnet.Spec = specFromProto(req.Spec)
	}

	return devnet
}

func metadataToProto(m *types.ResourceMeta) *v1.DevnetMetadata {
	return &v1.DevnetMetadata{
		Name:        m.Name,
		Generation:  m.Generation,
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
		Labels:      m.Labels,
		Annotations: m.Annotations,
	}
}

func metadataFromProto(pb *v1.DevnetMetadata) types.ResourceMeta {
	if pb == nil {
		return types.ResourceMeta{}
	}

	m := types.ResourceMeta{
		Name:        pb.Name,
		Generation:  pb.Generation,
		Labels:      pb.Labels,
		Annotations: pb.Annotations,
	}

	if pb.CreatedAt != nil {
		m.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		m.UpdatedAt = pb.UpdatedAt.AsTime()
	}

	return m
}

func specToProto(s *types.DevnetSpec) *v1.DevnetSpec {
	return &v1.DevnetSpec{
		Plugin:      s.Plugin,
		NetworkType: s.NetworkType,
		Validators:  int32(s.Validators),
		FullNodes:   int32(s.FullNodes),
		Mode:        s.Mode,
		SdkVersion:  s.BinarySource.Version,
		GenesisPath: s.GenesisPath,
		SnapshotUrl: s.SnapshotURL,
	}
}

func specFromProto(pb *v1.DevnetSpec) types.DevnetSpec {
	if pb == nil {
		return types.DevnetSpec{}
	}

	return types.DevnetSpec{
		Plugin:      pb.Plugin,
		NetworkType: pb.NetworkType,
		Validators:  int(pb.Validators),
		FullNodes:   int(pb.FullNodes),
		Mode:        pb.Mode,
		GenesisPath: pb.GenesisPath,
		SnapshotURL: pb.SnapshotUrl,
		BinarySource: types.BinarySource{
			Version: pb.SdkVersion,
		},
	}
}

func statusToProto(s *types.DevnetStatus) *v1.DevnetStatus {
	return &v1.DevnetStatus{
		Phase:           s.Phase,
		Nodes:           int32(s.Nodes),
		ReadyNodes:      int32(s.ReadyNodes),
		CurrentHeight:   s.CurrentHeight,
		SdkVersion:      s.SDKVersion,
		LastHealthCheck: timestamppb.New(s.LastHealthCheck),
		Message:         s.Message,
	}
}

func statusFromProto(pb *v1.DevnetStatus) types.DevnetStatus {
	if pb == nil {
		return types.DevnetStatus{}
	}

	s := types.DevnetStatus{
		Phase:         pb.Phase,
		Nodes:         int(pb.Nodes),
		ReadyNodes:    int(pb.ReadyNodes),
		CurrentHeight: pb.CurrentHeight,
		SDKVersion:    pb.SdkVersion,
		Message:       pb.Message,
	}

	if pb.LastHealthCheck != nil {
		s.LastHealthCheck = pb.LastHealthCheck.AsTime()
	}

	return s
}
