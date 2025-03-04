/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodelabels

import (
	"fmt"

	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/featureflag"
	"k8s.io/kops/util/pkg/reflectutils"
)

const (
	RoleLabelName15           = "kubernetes.io/role"
	RoleMasterLabelValue15    = "master"
	RoleAPIServerLabelValue15 = "api-server"
	RoleNodeLabelValue15      = "node"

	RoleLabelMaster16    = "node-role.kubernetes.io/master"
	RoleLabelAPIServer16 = "node-role.kubernetes.io/api-server"
	RoleLabelNode16      = "node-role.kubernetes.io/node"

	RoleLabelControlPlane20 = "node-role.kubernetes.io/control-plane"
)

// BuildNodeLabels returns the node labels for the specified instance group
// This moved from the kubelet to a central controller in kubernetes 1.16
func BuildNodeLabels(cluster *api.Cluster, instanceGroup *api.InstanceGroup) (map[string]string, error) {
	isControlPlane := false
	isAPIServer := false
	isNode := false
	switch instanceGroup.Spec.Role {
	case api.InstanceGroupRoleControlPlane:
		isControlPlane = true
	case api.InstanceGroupRoleAPIServer:
		isAPIServer = true
	case api.InstanceGroupRoleNode:
		isNode = true
	case api.InstanceGroupRoleBastion:
		// no labels to add
	default:
		return nil, fmt.Errorf("unhandled instanceGroup role %q", instanceGroup.Spec.Role)
	}

	// Merge KubeletConfig for NodeLabels
	c := &api.KubeletConfigSpec{}
	if isControlPlane {
		reflectutils.JSONMergeStruct(c, cluster.Spec.ControlPlaneKubelet)
	} else {
		reflectutils.JSONMergeStruct(c, cluster.Spec.Kubelet)
	}

	if instanceGroup.Spec.Kubelet != nil {
		reflectutils.JSONMergeStruct(c, instanceGroup.Spec.Kubelet)
	}

	nodeLabels := c.NodeLabels

	if isAPIServer || isControlPlane {
		if nodeLabels == nil {
			nodeLabels = make(map[string]string)
		}
		// Note: featureflag is not available here - we're in kops-controller.
		// We keep the featureflag as a placeholder to change the logic;
		// when we drop the featureflag we should just always include the label, even for
		// full control-plane nodes.
		if isAPIServer || featureflag.APIServerNodes.Enabled() {
			nodeLabels[RoleLabelAPIServer16] = ""
		}
		if cluster.IsKubernetesLT("1.24") {
			nodeLabels[RoleLabelName15] = RoleAPIServerLabelValue15
		}
	}

	if isNode {
		if nodeLabels == nil {
			nodeLabels = make(map[string]string)
		}
		nodeLabels[RoleLabelNode16] = ""
		if cluster.IsKubernetesLT("1.24") {
			nodeLabels[RoleLabelName15] = RoleNodeLabelValue15
		}
	}

	if isControlPlane {
		if nodeLabels == nil {
			nodeLabels = make(map[string]string)
		}
		for label, value := range BuildMandatoryControlPlaneLabels() {
			nodeLabels[label] = value
		}
		if cluster.IsKubernetesLT("1.24") {
			nodeLabels[RoleLabelMaster16] = ""
			nodeLabels[RoleLabelName15] = RoleMasterLabelValue15
		}
	}

	for k, v := range instanceGroup.Spec.NodeLabels {
		if nodeLabels == nil {
			nodeLabels = make(map[string]string)
		}
		nodeLabels[k] = v
	}

	if instanceGroup.Spec.Manager == api.InstanceManagerKarpenter {
		nodeLabels["karpenter.sh/provisioner-name"] = instanceGroup.ObjectMeta.Name
	}

	return nodeLabels, nil
}

// BuildMandatoryControlPlaneLabels returns the list of labels all CP nodes must have
func BuildMandatoryControlPlaneLabels() map[string]string {
	nodeLabels := make(map[string]string)
	nodeLabels[RoleLabelControlPlane20] = ""
	nodeLabels["kops.k8s.io/kops-controller-pki"] = ""
	nodeLabels["node.kubernetes.io/exclude-from-external-load-balancers"] = ""
	return nodeLabels
}
