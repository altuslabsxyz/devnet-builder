package server

import (
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
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

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:       req.Name,
			Namespace:  namespace,
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

// ApplyRequestToDevnet converts an ApplyDevnetRequest to a domain Devnet.
func ApplyRequestToDevnet(req *v1.ApplyDevnetRequest) *types.Devnet {
	now := time.Now()

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:        req.Name,
			Generation:  1,
			CreatedAt:   now,
			UpdatedAt:   now,
			Labels:      req.Labels,
			Annotations: req.Annotations,
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

// specsEqual compares a domain DevnetSpec with a proto DevnetSpec for equality.
func specsEqual(a types.DevnetSpec, b *v1.DevnetSpec) bool {
	if b == nil {
		return false
	}
	return a.Plugin == b.Plugin &&
		a.NetworkType == b.NetworkType &&
		a.Validators == int(b.Validators) &&
		a.FullNodes == int(b.FullNodes) &&
		a.Mode == b.Mode &&
		a.BinarySource.Version == b.SdkVersion &&
		a.GenesisPath == b.GenesisPath &&
		a.SnapshotURL == b.SnapshotUrl &&
		a.RPCURL == b.RpcUrl &&
		a.ForkNetwork == b.ForkNetwork &&
		a.ChainID == b.ChainId
}

// labelsEqual compares two label maps for equality.
func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func metadataToProto(m *types.ResourceMeta) *v1.DevnetMetadata {
	return &v1.DevnetMetadata{
		Name:        m.Name,
		Namespace:   m.Namespace,
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
		Namespace:   pb.Namespace,
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
		RpcUrl:      s.RPCURL,
		ForkNetwork: s.ForkNetwork,
		ChainId:     s.ChainID,
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
		RPCURL:      pb.RpcUrl,
		ForkNetwork: pb.ForkNetwork,
		ChainID:     pb.ChainId,
		BinarySource: types.BinarySource{
			Version: pb.SdkVersion,
		},
	}
}

func statusToProto(s *types.DevnetStatus) *v1.DevnetStatus {
	// Convert conditions
	var conditions []*v1.Condition
	for _, c := range s.Conditions {
		conditions = append(conditions, &v1.Condition{
			Type:               c.Type,
			Status:             c.Status,
			LastTransitionTime: timestamppb.New(c.LastTransitionTime),
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}

	// Convert events
	var events []*v1.Event
	for _, e := range s.Events {
		events = append(events, &v1.Event{
			Timestamp: timestamppb.New(e.Timestamp),
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Component: e.Component,
		})
	}

	return &v1.DevnetStatus{
		Phase:           s.Phase,
		Nodes:           int32(s.Nodes),
		ReadyNodes:      int32(s.ReadyNodes),
		CurrentHeight:   s.CurrentHeight,
		SdkVersion:      s.SDKVersion,
		LastHealthCheck: timestamppb.New(s.LastHealthCheck),
		Message:         s.Message,
		Conditions:      conditions,
		Events:          events,
	}
}

func statusFromProto(pb *v1.DevnetStatus) types.DevnetStatus {
	if pb == nil {
		return types.DevnetStatus{}
	}

	// Convert conditions
	var conditions []types.Condition
	for _, c := range pb.Conditions {
		cond := types.Condition{
			Type:    c.Type,
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		}
		if c.LastTransitionTime != nil {
			cond.LastTransitionTime = c.LastTransitionTime.AsTime()
		}
		conditions = append(conditions, cond)
	}

	// Convert events
	var events []types.Event
	for _, e := range pb.Events {
		evt := types.Event{
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Component: e.Component,
		}
		if e.Timestamp != nil {
			evt.Timestamp = e.Timestamp.AsTime()
		}
		events = append(events, evt)
	}

	s := types.DevnetStatus{
		Phase:         pb.Phase,
		Nodes:         int(pb.Nodes),
		ReadyNodes:    int(pb.ReadyNodes),
		CurrentHeight: pb.CurrentHeight,
		SDKVersion:    pb.SdkVersion,
		Message:       pb.Message,
		Conditions:    conditions,
		Events:        events,
	}

	if pb.LastHealthCheck != nil {
		s.LastHealthCheck = pb.LastHealthCheck.AsTime()
	}

	return s
}

// =============================================================================
// Upgrade converters
// =============================================================================

// UpgradeToProto converts a domain Upgrade to a proto Upgrade.
func UpgradeToProto(u *types.Upgrade) *v1.Upgrade {
	if u == nil {
		return nil
	}

	return &v1.Upgrade{
		Metadata: upgradeMetadataToProto(&u.Metadata),
		Spec:     upgradeSpecToProto(&u.Spec),
		Status:   upgradeStatusToProto(&u.Status),
	}
}

// UpgradeFromProto converts a proto Upgrade to a domain Upgrade.
func UpgradeFromProto(pb *v1.Upgrade) *types.Upgrade {
	if pb == nil {
		return nil
	}

	return &types.Upgrade{
		Metadata: upgradeMetadataFromProto(pb.Metadata),
		Spec:     upgradeSpecFromProto(pb.Spec),
		Status:   upgradeStatusFromProto(pb.Status),
	}
}

// CreateUpgradeRequestToUpgrade converts a CreateUpgradeRequest to a domain Upgrade.
func CreateUpgradeRequestToUpgrade(req *v1.CreateUpgradeRequest) *types.Upgrade {
	now := time.Now()

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{
			Name:       req.Name,
			Namespace:  namespace,
			Generation: 1,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhasePending,
		},
	}

	if req.Spec != nil {
		upgrade.Spec = upgradeSpecFromProto(req.Spec)
	}

	return upgrade
}

func upgradeMetadataToProto(m *types.ResourceMeta) *v1.UpgradeMetadata {
	return &v1.UpgradeMetadata{
		Name:       m.Name,
		Namespace:  m.Namespace,
		Generation: m.Generation,
		CreatedAt:  timestamppb.New(m.CreatedAt),
		UpdatedAt:  timestamppb.New(m.UpdatedAt),
	}
}

func upgradeMetadataFromProto(pb *v1.UpgradeMetadata) types.ResourceMeta {
	if pb == nil {
		return types.ResourceMeta{}
	}

	m := types.ResourceMeta{
		Name:       pb.Name,
		Namespace:  pb.Namespace,
		Generation: pb.Generation,
	}

	if pb.CreatedAt != nil {
		m.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		m.UpdatedAt = pb.UpdatedAt.AsTime()
	}

	return m
}

func upgradeSpecToProto(s *types.UpgradeSpec) *v1.UpgradeSpec {
	return &v1.UpgradeSpec{
		DevnetRef:    s.DevnetRef,
		UpgradeName:  s.UpgradeName,
		TargetHeight: s.TargetHeight,
		NewBinary:    binarySourceToProto(&s.NewBinary),
		WithExport:   s.WithExport,
		AutoVote:     s.AutoVote,
	}
}

func upgradeSpecFromProto(pb *v1.UpgradeSpec) types.UpgradeSpec {
	if pb == nil {
		return types.UpgradeSpec{}
	}

	return types.UpgradeSpec{
		DevnetRef:    pb.DevnetRef,
		UpgradeName:  pb.UpgradeName,
		TargetHeight: pb.TargetHeight,
		NewBinary:    binarySourceFromProto(pb.NewBinary),
		WithExport:   pb.WithExport,
		AutoVote:     pb.AutoVote,
	}
}

func upgradeStatusToProto(s *types.UpgradeStatus) *v1.UpgradeStatus {
	return &v1.UpgradeStatus{
		Phase:          s.Phase,
		ProposalId:     s.ProposalID,
		VotesReceived:  int32(s.VotesReceived),
		VotesRequired:  int32(s.VotesRequired),
		CurrentHeight:  s.CurrentHeight,
		PreExportPath:  s.PreExportPath,
		PostExportPath: s.PostExportPath,
		Message:        s.Message,
		Error:          s.Error,
	}
}

func upgradeStatusFromProto(pb *v1.UpgradeStatus) types.UpgradeStatus {
	if pb == nil {
		return types.UpgradeStatus{}
	}

	return types.UpgradeStatus{
		Phase:          pb.Phase,
		ProposalID:     pb.ProposalId,
		VotesReceived:  int(pb.VotesReceived),
		VotesRequired:  int(pb.VotesRequired),
		CurrentHeight:  pb.CurrentHeight,
		PreExportPath:  pb.PreExportPath,
		PostExportPath: pb.PostExportPath,
		Message:        pb.Message,
		Error:          pb.Error,
	}
}

func binarySourceToProto(b *types.BinarySource) *v1.BinarySource {
	if b == nil {
		return nil
	}

	return &v1.BinarySource{
		Type:    b.Type,
		Version: b.Version,
		Url:     b.URL,
		Path:    b.Path,
	}
}

func binarySourceFromProto(pb *v1.BinarySource) types.BinarySource {
	if pb == nil {
		return types.BinarySource{}
	}

	return types.BinarySource{
		Type:    pb.Type,
		Version: pb.Version,
		URL:     pb.Url,
		Path:    pb.Path,
	}
}
