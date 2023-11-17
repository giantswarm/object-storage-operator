package azure

type AzureCluster struct {
	SubscriptionID string
	AzureIdentity  AzureIdentity
}

type AzureIdentity struct {
	Type         string
	ClientID     string
	TenantID     string
	ClientSecret ClientSecret
}

type ClientSecret struct {
	Namespace string
	Name      string
}
