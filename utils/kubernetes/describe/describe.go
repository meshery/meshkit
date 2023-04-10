package describe

// describe package provides a way to describe Kubernetes objects for the kubernetes Api

import (
	meshkitkube "github.com/layer5io/meshkit/utils/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/describe"
)

// Meshkit Describe uses functions exposed from here https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/describe/describe.go#L709

// DescriberOptions give control of which kubernetes object to describe.
type DescriberOptions struct {
	Name       string       // Name of the kubernetes obj
	Namespace  string       // Namespace of the kubernetes obj
	ShowEvents bool         // A boolean flag indicating whether to show events associated with the Kubernetes object or not.
	ChunkSize  int64        //Size of the chunk in which the Kubernetes object's output is written.
	Type       DescribeType //an integer value that represents the Kubernetes source that needs to be described
}

/*
DescribeType represents the Kubernetes Source that needs to be Described
The integer value of the DescribeType is used to get the corresponding GroupKind information of the resource
from the ResourceMap variable, which is then used to get the describer function for that resource type.
*/
type DescribeType int

const (
	Service = iota
	Pod
	Namespace
	Job
	CronJob
	Deployment
	DaemonSet
	ReplicaSet
	StatefulSet
	Secret
	ServiceAccount
	Node
	LimitRange
	ResourceQuota
	PersistentVolume
	PersistentVolumeClaim
	Endpoints
	ConfigMap
	PriorityClass
	Ingress
	Role
	ClusterRole
	RoleBinding
	ClusterRoleBinding
	NetworkPolicy
	ReplicationController
	CertificateSigningRequest
	EndpointSlice
)

/*
  The "ResourceMap" map associates each "DescribeType" with a corresponding
  Kubernetes GroupKind object.
  which are used to identify the Kubernetes API resources that need to be described
*/

var ResourceMap = map[DescribeType]schema.GroupKind{
	Pod:                       {Group: corev1.GroupName, Kind: "Pod"},
	Deployment:                {Group: appsv1.GroupName, Kind: "Deployment"},
	Job:                       {Group: batchv1.GroupName, Kind: "Job"},
	CronJob:                   {Group: batchv1.GroupName, Kind: "CronJob"},
	StatefulSet:               {Group: appsv1.GroupName, Kind: "StatefulSet"},
	DaemonSet:                 {Group: appsv1.GroupName, Kind: "DaemonSet"},
	ReplicaSet:                {Group: appsv1.GroupName, Kind: "ReplicaSet"},
	Secret:                    {Group: corev1.GroupName, Kind: "Secret"},
	Service:                   {Group: corev1.GroupName, Kind: "Service"},
	ServiceAccount:            {Group: corev1.GroupName, Kind: "ServiceAccount"},
	Node:                      {Group: corev1.GroupName, Kind: "Node"},
	LimitRange:                {Group: corev1.GroupName, Kind: "LimitRange"},
	ResourceQuota:             {Group: corev1.GroupName, Kind: "ResourceQuota"},
	PersistentVolume:          {Group: corev1.GroupName, Kind: "PersistentVolume"},
	PersistentVolumeClaim:     {Group: corev1.GroupName, Kind: "PersistentVolumeClaim"},
	Namespace:                 {Group: corev1.GroupName, Kind: "Namespace"},
	Endpoints:                 {Group: corev1.GroupName, Kind: "Endpoints"},
	ConfigMap:                 {Group: corev1.GroupName, Kind: "ConfigMap"},
	PriorityClass:             {Group: corev1.GroupName, Kind: "PriorityClass"},
	Ingress:                   {Group: networkingv1.GroupName, Kind: "Ingress"},
	Role:                      {Group: rbacv1.GroupName, Kind: "Role"},
	ClusterRole:               {Group: rbacv1.GroupName, Kind: "ClusterRole"},
	RoleBinding:               {Group: rbacv1.GroupName, Kind: "RoleBinding"},
	ClusterRoleBinding:        {Group: rbacv1.GroupName, Kind: "ClusterRoleBinding"},
	NetworkPolicy:             {Group: networkingv1.GroupName, Kind: "NetworkPolicy"},
	ReplicationController:     {Group: corev1.GroupName, Kind: "ReplicationController"},
	CertificateSigningRequest: {Group: certificatesv1beta1.GroupName, Kind: "CertificateSigningRequest"},
	EndpointSlice:             {Group: discoveryv1.GroupName, Kind: "EndpointSlice"},
}

/*
The Describe() takes in a Kubernetes client and options for describing a particular Kubernetes resource.
It retrieves the GroupKind object associated with the specified "DescribeType" from the ResourceMap,
and then calls a corresponding "describer" function to retrieve the description of the specified Kubernetes resource
*/
func Describe(client *meshkitkube.Client, options DescriberOptions) (string, error) {
	// getting schema.GroupKind from Resource map
	kind := ResourceMap[options.Type]
	describer, ok := describe.DescriberFor(kind, &client.RestConfig)
	if !ok {
		return "", ErrGetDescriberFunc()
	}
	describerSetting := describe.DescriberSettings{
		ShowEvents: options.ShowEvents,
		ChunkSize:  options.ChunkSize,
	}
	//calls a corresponding "describer" function to retrieve the description of the specified Kubernetes resource
	output, err := describer.Describe(options.Namespace, options.Name, describerSetting)
	if err != nil {
		return "", err
	}
	/*
		The output returned includes information such as the resource's metadata (name, namespace.)
		and other details such as the resource's specifications, configuration, and associated events if ShowEvents option is set to true.
	*/
	return output, nil
}
