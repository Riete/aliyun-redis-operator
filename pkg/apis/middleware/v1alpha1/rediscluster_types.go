package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RedisExporterImage string = "riet/redis_exporter:v0.34.1"
	RedisExporterPort  int32  = 9121
	RedisClusterImage3 string = "riet/redis-cluster:3.2.12"
	RedisClusterImage4 string = "riet/redis-cluster:4.0.14"
	RedisTribImage     string = "riet/redis-trib:v1.0.0"
	BusyBoxImage       string = "riet/busybox"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisClusterSpec defines the desired state of RedisCluster
type RedisClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Replicas       int32                       `json:"replicas"`
	MajorVersion   int                         `json:"majorVersion"`
	Env            []corev1.EnvVar             `json:"env,omitempty"`
	Resources      corev1.ResourceRequirements `json:"resources,omitempty"`
	StorageClass   string                      `json:"storageClass,omitempty"`
	StorageSize    string                      `json:"storageSize,omitempty"`
	Isolated       bool                        `json:"isolated,omitempty"`
	Metrics        bool                        `json:"metrics,omitempty"`
	MasterSchedule bool                        `json:"masterSchedule,omitempty"`
}

// RedisClusterStatus defines the observed state of RedisCluster
type RedisClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisCluster is the Schema for the redisclusters API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=redisclusters,scope=Namespaced
type RedisCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisClusterSpec   `json:"spec,omitempty"`
	Status RedisClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisClusterList contains a list of RedisCluster
type RedisClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisCluster{}, &RedisClusterList{})
}
