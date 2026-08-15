package v1alpha1

// ObjectRef points to another AgentOrgs resource in the same namespace.
// +kubebuilder:object:generate=true
type ObjectRef struct {
	Kind string `json:"kind"` // Member / Group / Collaboration / Policy
	Name string `json:"name"` // resource name
}

// ConfigRef points to a ConfigMap key.
// +kubebuilder:object:generate=true
type ConfigRef struct {
	Name string `json:"name"`          // ConfigMap name
	Key  string `json:"key,omitempty"` // key inside the ConfigMap
}

// ProviderBinding selects an external provider implementation.
// +kubebuilder:object:generate=true
type ProviderBinding struct {
	Provider string    `json:"provider"` // provider name, e.g. openclaw / matrix / minio
	Config   ConfigRef `json:"config"`   // provider config in a ConfigMap
}
