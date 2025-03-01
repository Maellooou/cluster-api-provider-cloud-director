package controllers

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/vmware/cloud-provider-for-cloud-director/pkg/vcdsdk"
	infrav1beta3 "github.com/vmware/cluster-api-provider-cloud-director/api/v1beta3"
	rdeType "github.com/vmware/cluster-api-provider-cloud-director/pkg/vcdtypes/rde_type_1_1_0"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const tkgVersionLabel = "TKGVERSION"

func getTKGVersion(cluster *clusterv1.Cluster) string {
	annotationsMap := cluster.GetAnnotations()
	if tkgVersion, exists := annotationsMap[tkgVersionLabel]; exists {
		return tkgVersion
	}
	return ""
}

// filterTypeMetaAndObjectMetaFromK8sObjectMap is a helper function to remove extraneous contents in "objectmeta" and "typemeta"
//
//	keys. The function moves name and namespace from "objectmeta" key to "metadata" key and moves all the keys from "typemeta"
//	key to objMap
func filterTypeMetaAndObjectMetaFromK8sObjectMap(objMap map[string]interface{}) error {
	if _, ok := objMap["typemeta"]; ok {
		typeMetaMap, ok := objMap["typemeta"].(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("failed to convert typemeta [%v] to map[interface{}]interface{}", objMap["typemeta"])
		}
		// move contents of typeMetaMap to objMap. This preserves keys like apiVersion and Kind
		for k, v := range typeMetaMap {
			objMap[k.(string)] = v
		}
		delete(objMap, "typemeta")
	}

	if _, ok := objMap["objectmeta"]; ok {
		objectMetaMap, ok := objMap["objectmeta"].(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("failed to convert objectmeta [%v] to map[interface{}]interface{}", objMap["objectmeta"])
		}
		// remove all keys from objectMetaMap except for name and namespace.
		for k := range objectMetaMap {
			if k.(string) != "name" && k.(string) != "namespace" {
				delete(objectMetaMap, k)
			}
		}
		// preserve name and namespace of the object as part of "metadata"
		objMap["metadata"] = objectMetaMap
		delete(objMap, "objectmeta")
	}
	return nil
}

func yamlWithoutStatus(obj interface{}) (string, error) {
	// Redact the password and refresh token
	// get yaml string for obj
	objInByteArr, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal object: [%v]", err)
	}

	objMap := make(map[string]interface{})
	if err := yaml.Unmarshal(objInByteArr, &objMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal object to map[string]interface{}: [%v]", err)
	}

	// delete status key
	if _, ok := objMap["status"]; ok {
		delete(objMap, "status")
	}

	err = filterTypeMetaAndObjectMetaFromK8sObjectMap(objMap)
	if err != nil {
		return "", fmt.Errorf("failed to remove type meta and object meta from kubernetes object [%v]: [%v]", objMap, err)
	}

	// marshal back to a string
	output, err := yaml.Marshal(objMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal modified object: [%v]", err)
	}
	return string(output), nil
}

func getK8sObjectStatus(obj interface{}) (string, error) {
	// Redact the password and refresh token
	// get yaml string for obj
	objInByteArr, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal object: [%v]", err)
	}

	objMap := make(map[string]interface{})
	if err := yaml.Unmarshal(objInByteArr, &objMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal object to map[string]interface{}: [%v]", err)
	}

	// delete spec key
	if _, ok := objMap["spec"]; ok {
		delete(objMap, "spec")
	}

	err = filterTypeMetaAndObjectMetaFromK8sObjectMap(objMap)
	if err != nil {
		return "", fmt.Errorf("failed to remove type meta and object meta from kubernetes object [%v]: [%v]", objMap, err)
	}

	// marshal back to a string
	output, err := yaml.Marshal(objMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal modified object: [%v]", err)
	}
	return string(output), nil
}

func getOrgByName(client *vcdsdk.Client, orgName string) (*govcd.Org, error) {
	org, err := client.VCDClient.GetOrgByName(orgName)
	if err != nil {
		return nil, fmt.Errorf("failed to get org by name [%s]: [%v]", orgName, err)
	}
	if org == nil || org.Org == nil {
		return nil, fmt.Errorf("found nil org when getting org by name [%s]", orgName)
	}
	return org, nil
}

func getOrgByID(client *vcdsdk.Client, orgID string) (*govcd.Org, error) {
	org, err := client.VCDClient.GetOrgById(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get org by ID [%s]: [%v]", orgID, err)
	}
	if org == nil || org.Org == nil {
		return nil, fmt.Errorf("found nil org when getting org by ID [%s]", orgID)
	}
	return org, nil
}

func getOvdcByID(client *vcdsdk.Client, orgName string, ovdcID string) (*govcd.Vdc, error) {
	org, err := getOrgByName(client, orgName)
	if err != nil {
		return nil, fmt.Errorf("error occurred when getting ovdc by ID [%s]: [%v]", ovdcID, err)
	}
	ovdc, err := org.GetVDCById(ovdcID, true)
	if err != nil {
		if err == govcd.ErrorEntityNotFound {
			return nil, err
		}
		return nil, fmt.Errorf("fail to get ovdc by ID [%s]: [%v]", ovdcID, err)
	}
	if ovdc == nil || ovdc.Vdc == nil {
		return nil, fmt.Errorf("found nil ovdc when getting org by ID [%s]", ovdcID)
	}
	return ovdc, nil
}

func getOvdcByName(client *vcdsdk.Client, orgName string, ovdcName string) (*govcd.Vdc, error) {
	org, err := getOrgByName(client, orgName)
	if err != nil {
		return nil, fmt.Errorf("error occurred when getting ovdc by Name [%s]: [%v]", ovdcName, err)
	}
	ovdc, err := org.GetVDCByName(ovdcName, true)
	if err != nil {
		if err == govcd.ErrorEntityNotFound {
			return nil, err
		}
		return nil, fmt.Errorf("fail to get ovdc by Name [%s]: [%v]", ovdcName, err)
	}
	if ovdc == nil || ovdc.Vdc == nil {
		return nil, fmt.Errorf("found nil ovdc when getting org by Name [%s]", ovdcName)
	}
	return ovdc, nil
}

// Todo: Yan - Implement this function in the future
// Insert vcdResource into vcdcluster.status.VcdResourceMap.
// It should be the uniform function for all the types - org, ovdc, catalog, etc
func insertVcdResourceIntoVcdCluster(vcdCluster *infrav1beta3.VCDCluster, vcdResourceType string, resourceID string, resourceName string) error {
	return nil
}

// Todo: Yan - Implement this function in the future
// Insert vcdResource into vcdcluster.status.VcdResourceMap
// It should be the uniform function for all the types - org, ovdc, catalog, etc
func getVcdResourceFromVcdCluster(vcdCluster *infrav1beta3.VCDCluster, vcdResourceType string) ([]infrav1beta3.VCDResource, error) {
	return nil, nil
}

// Todo: Yan - Implement this function in the future
// Update the existing vcdResource into vcdcluster.status.VcdResourceMap.
// It should be the uniform function for all the types - org, ovdc, catalog, etc
func updateVdcResourceToVcdCluster(vcdCluster *infrav1beta3.VCDCluster, vcdResourceType string, resourceID string, resourceName string) error {
	switch vcdResourceType {
	case ResourceTypeOvdc:
		resourceList := vcdCluster.Status.VcdResourceMap.Ovdcs
		if resourceList == nil {
			resourceList = []infrav1beta3.VCDResource{}
		}
		for i, resource := range resourceList {
			if resource.ID == resourceID {
				if resource.Name != resourceName {
					resourceList[i].Name = resourceName
					vcdCluster.Status.VcdResourceMap.Ovdcs = resourceList
					return nil
				}
				return nil // Resource already exists with the same ID and name, no need for further action
			}
		}
		// Resource not found, add it to the list
		vcdCluster.Status.VcdResourceMap.Ovdcs = append(resourceList, infrav1beta3.VCDResource{
			ID:   resourceID,
			Name: resourceName,
		})
	default:
		return fmt.Errorf("unsupported VCD resource type: %s", vcdResourceType)
	}
	return nil
}

// Todo: Yan - Implement this function in the future
// Remove vcdResource from vcdcluster.status.VcdResourceMap.
// It should be the uniform function for all the types - org, ovdc, catalog, etc
func removeVcdResourceFromVcdCluster(vcdCluster *infrav1beta3.VCDCluster, vcdResourceType string, resourceID string) error {
	switch vcdResourceType {
	case ResourceTypeOvdc:
		resourceList := vcdCluster.Status.VcdResourceMap.Ovdcs
		for i, resource := range resourceList {
			if resource.ID == resourceID {
				resourceList = append(resourceList[:i], resourceList[i+1:]...)
				vcdCluster.Status.VcdResourceMap.Ovdcs = resourceList
				return nil
			}
		}
	default:
		return fmt.Errorf("unsupported VCD resource type: %s", vcdResourceType)
	}
	return fmt.Errorf("resource with ID %s not found in VCD cluster", resourceID)
}

// checkIfOvdcNameChange is used to check if ovdc name is changed during the CAPVCD provisioning process.
// Check vcdcluster.status.VcdResourceMap. Find the ovdc Object and fetch the ovdcID.
// Use the ovdcID to execute VCD API Call to get the ovdc in VCD.
// compare the oldOvdcName and newOvdcName.
// Return changed, vdc object, error.
func checkIfOvdcNameChange(vcdCluster *infrav1beta3.VCDCluster, client *vcdsdk.Client) (bool, *govcd.Vdc, error) {
	orgName := vcdCluster.Spec.Org
	ovdcSpecName := vcdCluster.Spec.Ovdc

	ovdcStatusName := vcdCluster.Status.Ovdc
	if ovdcStatusName == "" {
		ovdcStatusName = ovdcSpecName
	}

	ovdcID := ""
	nameChanged := false
	var ovdc *govcd.Vdc
	var err error

	if vcdCluster.Status.VcdResourceMap.Ovdcs != nil && len(vcdCluster.Status.VcdResourceMap.Ovdcs) > 0 {
		for _, ovdc := range vcdCluster.Status.VcdResourceMap.Ovdcs {
			if ovdc.Name == ovdcStatusName {
				ovdcID = ovdc.ID
			}
		}
	}

	// if ovdcID is not found in the vcdcluster.status.resourceSet, use ovdcStatusName instead to get the OVDC.
	if ovdcID == "" {
		ovdc, err = getOvdcByName(client, orgName, ovdcStatusName)
		if err != nil {
			return nameChanged, nil, fmt.Errorf("error occurred while checking if ovdcSpecName has changed; failed to get ovdc by Name [%s]: [%v]", ovdcStatusName, err)
		}
		//ovdcID is empty, which means we must add ovdc.Id and ovdc.Name to the resourceMap.ovdcs
		nameChanged = true
	} else {
		ovdc, err = getOvdcByID(client, orgName, ovdcID)
		if err != nil {
			if err == govcd.ErrorEntityNotFound {
				if removeErr := removeVcdResourceFromVcdCluster(vcdCluster, ResourceTypeOvdc, ovdcID); removeErr != nil {
					return false, nil, fmt.Errorf("error occurred while removing resource [%s] vcdResource from vcdcluster.status.vcdResourceMap: [%v]", ovdcID, removeErr)
				}
				return nameChanged, nil, fmt.Errorf("error occurred while checking if ovdcSpecName has changed; failed to get ovdc by ID [%s]: [%v]", ovdcID, err)
			}
			return nameChanged, nil, fmt.Errorf("error occurred while checking if ovdcSpecName has changed: [%v]", err)
		}
		nameChanged = ovdc.Vdc.Name != ovdcSpecName
	}
	return nameChanged, ovdc, nil
}

func getAllMachineDeploymentsForCluster(ctx context.Context, cli client.Client, c clusterv1.Cluster) (*clusterv1.MachineDeploymentList, error) {
	mdListLabels := map[string]string{clusterv1.ClusterNameLabel: c.Name}
	mdList := &clusterv1.MachineDeploymentList{}
	if err := cli.List(ctx, mdList, client.InNamespace(c.Namespace), client.MatchingLabels(mdListLabels)); err != nil {
		return nil, errors.Wrapf(err, "error getting machine deployments for the cluster [%s]", c.Name)
	}
	return mdList, nil
}

func getAllKubeadmControlPlaneForCluster(ctx context.Context, cli client.Client, c clusterv1.Cluster) (*kcpv1.KubeadmControlPlaneList, error) {
	kcpListLabels := map[string]string{clusterv1.ClusterNameLabel: c.Name}
	kcpList := &kcpv1.KubeadmControlPlaneList{}

	if err := cli.List(ctx, kcpList, client.InNamespace(c.Namespace), client.MatchingLabels(kcpListLabels)); err != nil {
		return nil, errors.Wrapf(err, "error getting all kubeadm control planes for the cluster [%s]", c.Name)
	}
	return kcpList, nil
}

func getAllCRSBindingForCluster(ctx context.Context, cli client.Client,
	c clusterv1.Cluster) (*addonsv1.ClusterResourceSetBindingList, error) {
	crsBindingList := &addonsv1.ClusterResourceSetBindingList{}

	if err := cli.List(ctx, crsBindingList, client.InNamespace(c.Namespace)); err != nil {
		return nil, fmt.Errorf("unable to get ClusterResourceSetBindingList for cluster [%s/%s]: [%v]",
			c.Namespace, c.Name, err)
	}

	return crsBindingList, nil
}

func getVCDMachineTemplateFromKCP(ctx context.Context, cli client.Client, kcp kcpv1.KubeadmControlPlane) (*infrav1beta3.VCDMachineTemplate, error) {
	vcdMachineTemplateRef := kcp.Spec.MachineTemplate.InfrastructureRef
	vcdMachineTemplate := &infrav1beta3.VCDMachineTemplate{}
	vcdMachineTemplateKey := types.NamespacedName{
		Namespace: vcdMachineTemplateRef.Namespace,
		Name:      vcdMachineTemplateRef.Name,
	}
	if err := cli.Get(ctx, vcdMachineTemplateKey, vcdMachineTemplate); err != nil {
		return nil, fmt.Errorf("failed to get VCDMachineTemplate by name [%s] from KCP [%s]: [%v]", vcdMachineTemplateRef.Name, kcp.Name, err)
	}

	return vcdMachineTemplate, nil
}

func getVCDMachineTemplateFromMachineDeployment(ctx context.Context, cli client.Client, md clusterv1.MachineDeployment) (*infrav1beta3.VCDMachineTemplate, error) {
	vcdMachineTemplateRef := md.Spec.Template.Spec.InfrastructureRef
	vcdMachineTemplate := &infrav1beta3.VCDMachineTemplate{}
	vcdMachineTemplateKey := client.ObjectKey{
		Namespace: vcdMachineTemplateRef.Namespace,
		Name:      vcdMachineTemplateRef.Name,
	}
	if err := cli.Get(ctx, vcdMachineTemplateKey, vcdMachineTemplate); err != nil {
		return nil, fmt.Errorf("failed to get VCDMachineTemplate by name [%s] from machine deployment [%s]: [%v]", vcdMachineTemplateRef.Name, md.Name, err)
	}

	return vcdMachineTemplate, nil
}

func getMachineListFromCluster(ctx context.Context, cli client.Client, cluster clusterv1.Cluster) (*clusterv1.MachineList, error) {
	machineListLabels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := cli.List(ctx, machineList, client.InNamespace(cluster.Namespace), client.MatchingLabels(machineListLabels)); err != nil {
		return nil, errors.Wrapf(err, "error getting machine list for the cluster [%s]", cluster.Name)
	}
	return machineList, nil
}

func getVCDMachineTemplateByObjRef(ctx context.Context, cli client.Client, objRef v1.ObjectReference) (*infrav1beta3.VCDMachineTemplate, error) {
	vcdMachineTemplate := &infrav1beta3.VCDMachineTemplate{}
	vcdMachineTemplateKey := client.ObjectKey{
		Namespace: objRef.Namespace,
		Name:      objRef.Name,
	}
	if err := cli.Get(ctx, vcdMachineTemplateKey, vcdMachineTemplate); err != nil {
		return nil, fmt.Errorf("failed to get VCDMachineTemplate by ObjectReference [%v]: [%v]", objRef, err)
	}

	return vcdMachineTemplate, nil
}

func getKubeadmConfigTemplateByObjRef(ctx context.Context, cli client.Client, objRef v1.ObjectReference) (*v1beta1.KubeadmConfigTemplate, error) {
	kubeadmConfigTemplate := &v1beta1.KubeadmConfigTemplate{}
	kubeadmConfigTemplateKey := client.ObjectKey{
		Namespace: objRef.Namespace,
		Name:      objRef.Name,
	}
	if err := cli.Get(ctx, kubeadmConfigTemplateKey, kubeadmConfigTemplate); err != nil {
		return nil, fmt.Errorf("failed to get KubeadmConfigTemplate by ObjectReference [%v]: [%v]", objRef, err)
	}

	return kubeadmConfigTemplate, nil
}

func getAllMachinesInMachineDeployment(ctx context.Context, cli client.Client, machineDeployment clusterv1.MachineDeployment) (*clusterv1.MachineList, error) {
	machineListLabels := map[string]string{clusterv1.MachineDeploymentNameLabel: machineDeployment.Name}
	machineList := &clusterv1.MachineList{}
	if err := cli.List(ctx, machineList, client.InNamespace(machineDeployment.Namespace), client.MatchingLabels(machineListLabels)); err != nil {
		return nil, errors.Wrapf(err, "error getting machine list for the cluster [%s]", machineDeployment.Name)
	}
	return machineList, nil
}

func getAllMachinesInKCP(ctx context.Context, cli client.Client, kcp kcpv1.KubeadmControlPlane, clusterName string) ([]clusterv1.Machine, error) {
	machineListLabels := map[string]string{clusterv1.ClusterNameLabel: clusterName}
	machineList := &clusterv1.MachineList{}
	if err := cli.List(ctx, machineList, client.InNamespace(kcp.Namespace), client.MatchingLabels(machineListLabels)); err != nil {
		return nil, errors.Wrapf(err, "error getting machine list associated with KCP [%s]: [%v]", kcp.Name, err)
	}
	// TODO find a better way to find all machines in KCP
	machinesWithKCPOwnerRef := make([]clusterv1.Machine, 0)
	for _, m := range machineList.Items {
		for _, ref := range m.OwnerReferences {
			if ref.Kind == "KubeadmControlPlane" && ref.Name == kcp.Name {
				machinesWithKCPOwnerRef = append(machinesWithKCPOwnerRef, m)
				break
			}
		}
	}
	return machinesWithKCPOwnerRef, nil
}

func getNodePoolList(ctx context.Context, cli client.Client, cluster clusterv1.Cluster) ([]rdeType.NodePool, error) {
	nodePoolList := make([]rdeType.NodePool, 0)
	mds, err := getAllMachineDeploymentsForCluster(ctx, cli, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to query all machine deployments for the cluster [%s]: [%v]", cluster.Name, err)
	}
	for _, md := range mds.Items {
		// create a node pool for each machine deployment
		vcdMachineTemplate, err := getVCDMachineTemplateFromMachineDeployment(ctx, cli, md)
		if err != nil {
			return nil, fmt.Errorf("failed to get VCDMachineTemplate associated with the MachineDeployment [%s]: [%v]", md.Name, err)
		}
		// query all machines in machine deployment using machine deployment label
		machineList, err := getAllMachinesInMachineDeployment(ctx, cli, md)
		if err != nil {
			return nil, fmt.Errorf("failed to get MachineList for MachineDeployment [%s]: [%v]", md.Name, err)
		}
		nodeStatusMap := make(map[string]string)
		for _, machine := range machineList.Items {
			nodeStatusMap[machine.Name] = machine.Status.Phase
		}
		desiredReplicasCount := int32(0)
		if md.Spec.Replicas != nil {
			desiredReplicasCount = *md.Spec.Replicas
		}
		nodePool := rdeType.NodePool{
			Name:              md.Name,
			SizingPolicy:      vcdMachineTemplate.Spec.Template.Spec.SizingPolicy,
			PlacementPolicy:   vcdMachineTemplate.Spec.Template.Spec.PlacementPolicy,
			NvidiaGpuEnabled:  vcdMachineTemplate.Spec.Template.Spec.EnableNvidiaGPU,
			StorageProfile:    vcdMachineTemplate.Spec.Template.Spec.StorageProfile,
			DiskSizeMb:        int32(vcdMachineTemplate.Spec.Template.Spec.DiskSize.Value() / (1024 * 1024)),
			DesiredReplicas:   desiredReplicasCount,
			AvailableReplicas: md.Status.ReadyReplicas,
			NodeStatus:        nodeStatusMap,
		}
		nodePoolList = append(nodePoolList, nodePool)
	}

	kcpList, err := getAllKubeadmControlPlaneForCluster(ctx, cli, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to query all KubeadmControlPlane objects for the cluster [%s]: [%v]", cluster.Name, err)
	}
	for _, kcp := range kcpList.Items {
		// create a node pool for each kcp
		vcdMachineTemplate, err := getVCDMachineTemplateFromKCP(ctx, cli, kcp)
		if err != nil {
			return nil, fmt.Errorf("failed to get VCDMachineTemplate associated with KubeadmControlPlane [%s]: [%v]", kcp.Name, err)
		}
		// query all machines with the kcp
		machineArr, err := getAllMachinesInKCP(ctx, cli, kcp, cluster.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get Machines associated with the KubeadmControlPlane [%s]: [%v]", kcp.Name, err)
		}
		nodeStatusMap := make(map[string]string)
		for _, machine := range machineArr {
			nodeStatusMap[machine.Name] = machine.Status.Phase
		}
		desiredReplicaCount := int32(0)
		if kcp.Spec.Replicas != nil {
			desiredReplicaCount = *kcp.Spec.Replicas
		}
		nodePool := rdeType.NodePool{
			Name:              kcp.Name,
			SizingPolicy:      vcdMachineTemplate.Spec.Template.Spec.SizingPolicy,
			PlacementPolicy:   vcdMachineTemplate.Spec.Template.Spec.PlacementPolicy,
			NvidiaGpuEnabled:  vcdMachineTemplate.Spec.Template.Spec.EnableNvidiaGPU,
			StorageProfile:    vcdMachineTemplate.Spec.Template.Spec.StorageProfile,
			DiskSizeMb:        int32(vcdMachineTemplate.Spec.Template.Spec.DiskSize.Value() / (1024 * 1024)),
			DesiredReplicas:   desiredReplicaCount,
			AvailableReplicas: kcp.Status.ReadyReplicas,
			NodeStatus:        nodeStatusMap,
		}
		nodePoolList = append(nodePoolList, nodePool)
	}
	return nodePoolList, nil
}

func getK8sClusterObjects(ctx context.Context, cli client.Client, cluster clusterv1.Cluster, vcdCluster infrav1beta3.VCDCluster) ([]interface{}, error) {
	// Redacting username, password and refresh token from the UserCredentialsContext for security purposes.
	vcdCluster.Spec.UserCredentialsContext.Username = "***REDACTED***"
	vcdCluster.Spec.UserCredentialsContext.Password = "***REDACTED***"
	vcdCluster.Spec.UserCredentialsContext.RefreshToken = "***REDACTED***"
	capiYamlObjects := []interface{}{
		cluster,
		vcdCluster,
	}

	kcpList, err := getAllKubeadmControlPlaneForCluster(ctx, cli, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get all KCPs from Cluster object: [%v]", err)
	}

	mdList, err := getAllMachineDeploymentsForCluster(ctx, cli, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get all the MachineDeployments from Cluster: [%v]", err)
	}

	vcdMachineTemplateNameToObjRef := make(map[string]v1.ObjectReference)
	for _, kcp := range kcpList.Items {
		vcdMachineTemplateNameToObjRef[kcp.Spec.MachineTemplate.InfrastructureRef.Name] = kcp.Spec.MachineTemplate.InfrastructureRef
	}

	kubeadmConfigTemplateNameToObjRef := make(map[string]*v1.ObjectReference)
	for _, md := range mdList.Items {
		vcdMachineTemplateNameToObjRef[md.Spec.Template.Spec.InfrastructureRef.Name] = md.Spec.Template.Spec.InfrastructureRef
		kubeadmConfigTemplateNameToObjRef[md.Spec.Template.Spec.Bootstrap.ConfigRef.Name] = md.Spec.Template.Spec.Bootstrap.ConfigRef
	}

	vcdMachineTemplates := make([]*infrav1beta3.VCDMachineTemplate, 0)
	for _, objRef := range vcdMachineTemplateNameToObjRef {
		vcdMachineTemplate, err := getVCDMachineTemplateByObjRef(ctx, cli, objRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get VCDMachineTemplate by ObjectReference [%v]: [%v]", objRef, err)
		}
		vcdMachineTemplates = append(vcdMachineTemplates, vcdMachineTemplate)
	}

	kubeadmConfigTemplates := make([]*v1beta1.KubeadmConfigTemplate, 0)
	for _, objRef := range kubeadmConfigTemplateNameToObjRef {
		kubeadmConifgTemplate, err := getKubeadmConfigTemplateByObjRef(ctx, cli, *objRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get KubeadmConfigTemplate by ObjectReference [%v]: [%v]", objRef, err)
		}
		kubeadmConfigTemplates = append(kubeadmConfigTemplates, kubeadmConifgTemplate)
	}

	// add objects
	for _, vcdMachineTemplate := range vcdMachineTemplates {
		capiYamlObjects = append(capiYamlObjects, *vcdMachineTemplate)
	}
	for _, kubeadmConfigTemplate := range kubeadmConfigTemplates {
		capiYamlObjects = append(capiYamlObjects, *kubeadmConfigTemplate)
	}
	for _, kcp := range kcpList.Items {
		capiYamlObjects = append(capiYamlObjects, kcp)
	}
	for _, md := range mdList.Items {
		capiYamlObjects = append(capiYamlObjects, md)
	}
	return capiYamlObjects, nil
}

func getCapiYaml(ctx context.Context, cli client.Client, cluster clusterv1.Cluster, vcdCluster infrav1beta3.VCDCluster) (string, error) {
	capiYamlObjects, err := getK8sClusterObjects(ctx, cli, cluster, vcdCluster)
	if err != nil {
		return "", fmt.Errorf("failed to get k8s objects related to cluster [%s]: [%v]", cluster.Name, err)
	}
	yamlObjects := make([]string, len(capiYamlObjects))
	for idx, obj := range capiYamlObjects {
		yamlString, err := yamlWithoutStatus(obj)
		if err != nil {
			return "", fmt.Errorf("failed to convert object to yaml: [%v]", err)
		}
		yamlObjects[idx] = yamlString
	}

	return strings.Join(yamlObjects, "---\n"), nil

}

func getCapiStatusYaml(ctx context.Context, cli client.Client, cluster clusterv1.Cluster, vcdCluster infrav1beta3.VCDCluster) (string, error) {
	capiYamlObjects, err := getK8sClusterObjects(ctx, cli, cluster, vcdCluster)
	if err != nil {
		return "", fmt.Errorf("failed to get k8s objects related to cluster [%s]: [%v]", cluster.Name, err)
	}
	yamlObjects := make([]string, len(capiYamlObjects))
	for idx, obj := range capiYamlObjects {
		yamlStatusString, err := getK8sObjectStatus(obj)
		if err != nil {
			return "", fmt.Errorf("failed to extract status from kuberenets object: [%v]", err)
		}
		yamlObjects[idx] = yamlStatusString
	}
	return strings.Join(yamlObjects, "---\n"), nil
}

func getUserCredentialsForCluster(ctx context.Context, cli client.Client, definedCreds infrav1beta3.UserCredentialsContext) (infrav1beta3.UserCredentialsContext, error) {
	username, password, refreshToken := definedCreds.Username, definedCreds.Password, definedCreds.RefreshToken
	if definedCreds.SecretRef != nil {
		secretNamespacedName := types.NamespacedName{
			Name:      definedCreds.SecretRef.Name,
			Namespace: definedCreds.SecretRef.Namespace,
		}
		userCredsSecret := &v1.Secret{}
		if err := cli.Get(ctx, secretNamespacedName, userCredsSecret); err != nil {
			return infrav1beta3.UserCredentialsContext{}, errors.Wrapf(err, "error getting secret [%s] in namespace [%s]",
				secretNamespacedName.Name, secretNamespacedName.Namespace)
		}
		if b, exists := userCredsSecret.Data["username"]; exists {
			username = strings.TrimRight(string(b), "\n")
		}
		if b, exists := userCredsSecret.Data["password"]; exists {
			password = strings.TrimRight(string(b), "\n")
		}
		if b, exists := userCredsSecret.Data["refreshToken"]; exists {
			refreshToken = strings.TrimRight(string(b), "\n")
		}
	}
	userCredentials := infrav1beta3.UserCredentialsContext{
		Username:     username,
		Password:     password,
		RefreshToken: refreshToken,
	}

	return userCredentials, nil
}

// hasClusterReconciledToDesiredK8Version returns true if all the kubeadm control plane objects and machine deployments have
// reconciled to the desired kubernetes version, else returns false.
func hasClusterReconciledToDesiredK8Version(ctx context.Context, cli client.Client, clusterName string,
	kcpList *kcpv1.KubeadmControlPlaneList, mdList *clusterv1.MachineDeploymentList, expectedVersion string) (bool, error) {

	for _, kcp := range kcpList.Items {
		machines, err := getAllMachinesInKCP(ctx, cli, kcp, clusterName)
		if err != nil {
			return false, fmt.Errorf("failed to fetch machines for the kubeadm control plane object [%s] for cluster [%s]: [%v]", kcp.Name, clusterName, err)
		}
		for _, machine := range machines {
			if machine.Spec.Version != nil && *machine.Spec.Version != expectedVersion {
				return false, nil
			}
		}
	}

	for _, md := range mdList.Items {
		machineList, err := getAllMachinesInMachineDeployment(ctx, cli, md)
		if err != nil {
			return false, fmt.Errorf("failed to fetch machines for the machine deployment [%s] for cluster [%s]: [%v]", md.Name, clusterName, err)
		}
		for _, machine := range machineList.Items {
			if machine.Spec.Version != nil && *machine.Spec.Version != expectedVersion {
				return false, nil
			}
		}
	}
	return true, nil
}
