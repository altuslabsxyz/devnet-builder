package config

import (
	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

// ToProto converts YAMLDevnet to protobuf Devnet
func (d *YAMLDevnet) ToProto() *v1.Devnet {
	return &v1.Devnet{
		Metadata: d.metadataToProto(),
		Spec:     d.specToProto(),
		Status: &v1.DevnetStatus{
			Phase: "Pending",
		},
	}
}

func (d *YAMLDevnet) metadataToProto() *v1.DevnetMetadata {
	namespace := d.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return &v1.DevnetMetadata{
		Name:        d.Metadata.Name,
		Namespace:   namespace,
		Labels:      d.Metadata.Labels,
		Annotations: d.Metadata.Annotations,
	}
}

func (d *YAMLDevnet) specToProto() *v1.DevnetSpec {
	spec := &v1.DevnetSpec{
		Plugin:      d.Spec.Network,
		NetworkType: d.Spec.NetworkType,
		Validators:  int32(d.Spec.Validators),
		FullNodes:   int32(d.Spec.FullNodes),
		Mode:        d.Spec.Mode,
		SdkVersion:  d.Spec.NetworkVersion,
	}

	// Apply defaults
	if spec.Mode == "" {
		spec.Mode = "docker"
	}
	if spec.NetworkType == "" {
		spec.NetworkType = "mainnet"
	}
	if spec.Validators == 0 {
		spec.Validators = 1
	}

	return spec
}

// YAMLDevnetFromProto converts protobuf Devnet to YAMLDevnet
func YAMLDevnetFromProto(pb *v1.Devnet) YAMLDevnet {
	yaml := YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
	}

	if pb.Metadata != nil {
		yaml.Metadata = YAMLMetadata{
			Name:        pb.Metadata.Name,
			Namespace:   pb.Metadata.Namespace,
			Labels:      pb.Metadata.Labels,
			Annotations: pb.Metadata.Annotations,
		}
	}

	if pb.Spec != nil {
		yaml.Spec = YAMLDevnetSpec{
			Network:        pb.Spec.Plugin,
			NetworkType:    pb.Spec.NetworkType,
			NetworkVersion: pb.Spec.SdkVersion,
			Validators:     int(pb.Spec.Validators),
			FullNodes:      int(pb.Spec.FullNodes),
			Mode:           pb.Spec.Mode,
		}
	}

	return yaml
}

// ToCreateRequest converts to a CreateDevnetRequest
func (d *YAMLDevnet) ToCreateRequest() *v1.CreateDevnetRequest {
	namespace := d.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return &v1.CreateDevnetRequest{
		Name:      d.Metadata.Name,
		Namespace: namespace,
		Spec:      d.specToProto(),
		Labels:    d.Metadata.Labels,
	}
}
