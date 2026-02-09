package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v9"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"

	"github.com/giantswarm/object-storage-operator/api/v1alpha1"
)

const (
	// This is how the private zone must be named in Azure for the private endpoint to work.
	privateZoneID = "privatelink.blob.core.windows.net"
	vnetID        = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s"
	subnetID      = vnetID + "/subnets/%s"
)

func (s AzureObjectStorageAdapter) upsertPrivateEndpointARecords(ctx context.Context, bucket *v1alpha1.Bucket, privateEndpoint *armnetwork.PrivateEndpoint, storageAccountName string) (*armprivatedns.RecordSet, error) {
	s.logger.Info("Creating A record for private endpoint", "private-endpoint", *privateEndpoint.Name)

	ips := make([]string, 0)

	for _, dnsConfigs := range privateEndpoint.Properties.CustomDNSConfigs {
		if dnsConfigs.IPAddresses == nil {
			continue
		}
		for _, ip := range dnsConfigs.IPAddresses {
			ips = append(ips, *ip)
		}
	}

	aRecords := make([]*armprivatedns.ARecord, len(ips))
	for i, ip := range ips {
		aRecords[i] = &armprivatedns.ARecord{IPv4Address: &ip}
	}

	resp, err := s.recordSetsClient.CreateOrUpdate(
		ctx,
		s.cluster.GetResourceGroup(),
		privateZoneID,
		armprivatedns.RecordTypeA,
		storageAccountName,
		armprivatedns.RecordSet{
			Properties: &armprivatedns.RecordSetProperties{
				ARecords: aRecords,
				TTL:      to.Ptr(int64(time.Hour.Seconds())),
				Metadata: s.getBucketTags(bucket),
			},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	return &resp.RecordSet, nil
}

func (s AzureObjectStorageAdapter) upsertPrivateEndpoint(ctx context.Context, bucket *v1alpha1.Bucket, storageAccountName string) (*armnetwork.PrivateEndpoint, error) {
	// Create or Update Private endpoint
	pollersResp, err := s.privateEndpointsClient.BeginCreateOrUpdate(
		ctx,
		s.cluster.GetResourceGroup(),
		bucket.Spec.Name,
		armnetwork.PrivateEndpoint{
			Location: to.Ptr(s.cluster.GetRegion()),
			Properties: &armnetwork.PrivateEndpointProperties{
				CustomNetworkInterfaceName: to.Ptr(fmt.Sprintf("%s-nodes-nic", bucket.Spec.Name)),
				PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{
					{
						Name: to.Ptr(bucket.Spec.Name),
						Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: to.Ptr(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", s.cluster.GetSubscriptionID(), s.cluster.GetResourceGroup(), storageAccountName)),
							GroupIDs:             []*string{to.Ptr("blob")},
						},
					},
				},
				Subnet: &armnetwork.Subnet{
					ID: to.Ptr(s.subnetID()),
				},
			},
			Tags: s.getBucketTags(bucket),
		},
		nil,
	)

	if err != nil {
		return nil, err
	}
	resp, err := pollersResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.PrivateEndpoint, nil
}

func (s AzureObjectStorageAdapter) upsertPrivateZone(ctx context.Context, bucket *v1alpha1.Bucket) (*armprivatedns.PrivateZone, error) {
	pollersResp, err := s.privateZonesClient.BeginCreateOrUpdate(
		ctx,
		s.cluster.GetResourceGroup(),
		privateZoneID,
		armprivatedns.PrivateZone{
			// Private Zone DNS is a global resource
			Location: to.Ptr("Global"),
			Tags:     s.getBucketTags(bucket),
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	resp, err := pollersResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.PrivateZone, nil
}

func (s AzureObjectStorageAdapter) upsertVirtualNetworkLink(ctx context.Context, bucket *v1alpha1.Bucket) (*armprivatedns.VirtualNetworkLink, error) {
	pollersResp, err := s.virtualNetworkLinksClient.BeginCreateOrUpdate(
		ctx,
		s.cluster.GetResourceGroup(),
		privateZoneID,
		"giantswarm-observability",
		armprivatedns.VirtualNetworkLink{
			// Private Zone DNS is a global resource
			Location: to.Ptr("Global"),
			Properties: &armprivatedns.VirtualNetworkLinkProperties{
				RegistrationEnabled: to.Ptr(false),
				VirtualNetwork: &armprivatedns.SubResource{
					ID: to.Ptr(s.vnetID()),
				},
			},
			Tags: s.getBucketTags(bucket),
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	resp, err := pollersResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.VirtualNetworkLink, nil
}

func (s AzureObjectStorageAdapter) vnetID() string {
	return fmt.Sprintf(vnetID, s.cluster.GetSubscriptionID(), s.cluster.GetResourceGroup(), s.cluster.GetVNetName())
}

func (s AzureObjectStorageAdapter) subnetID() string {
	return fmt.Sprintf(subnetID, s.cluster.GetSubscriptionID(), s.cluster.GetResourceGroup(), s.cluster.GetVNetName(), "node-subnet")
}
