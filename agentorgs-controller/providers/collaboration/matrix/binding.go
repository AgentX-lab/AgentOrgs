package matrix

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadConfigMapKey reads one key from a ConfigMap. Empty key defaults to the
// first data entry when only one exists; otherwise returns "".
func ReadConfigMapKey(ctx context.Context, c client.Client, namespace, name, key string) (string, error) {
	if name == "" {
		return "", nil
	}
	var cm corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &cm); err != nil {
		return "", err
	}
	if cm.Data == nil {
		return "", nil
	}
	if key != "" {
		return cm.Data[key], nil
	}
	if len(cm.Data) == 1 {
		for _, v := range cm.Data {
			return v, nil
		}
	}
	return "", nil
}
