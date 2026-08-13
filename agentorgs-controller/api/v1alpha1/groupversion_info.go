// Package v1alpha1 defines AgentOrgs Kubernetes API types.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	GroupName = "agentorgs.io"
	Version   = "v1alpha1"
)

var (
	GroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme = SchemeBuilder.AddToScheme
)

// Condition reports resource health.
// +kubebuilder:object:generate=true
type Condition struct {
	Type               string             `json:"type"`
	Status             metav1.ConditionStatus `json:"status"`
	Reason             string             `json:"reason,omitempty"`
	Message            string             `json:"message,omitempty"`
	LastTransitionTime metav1.Time        `json:"lastTransitionTime,omitempty"`
}
