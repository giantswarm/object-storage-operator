package flags

import "sigs.k8s.io/controller-runtime/pkg/client"

type ManagementCluster struct {
	BaseDomain string
	Name       string
	Namespace  string
	Provider   string
	Region     string
}

func (m ManagementCluster) ToObjectKey(name string, namespace string) client.ObjectKey {
	return client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
}
