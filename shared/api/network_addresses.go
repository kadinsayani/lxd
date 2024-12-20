package api

// NetworkAllocations used for displaying network addresses used by a consuming entity
// e.g, instance, network forward, load-balancer, network...
//
// swagger:model
//
// API extension: network_allocations.
type NetworkAllocations struct {
	// The network address of the allocation (in CIDR format)
	// Example: 192.0.2.1/24
	Address string `json:"addresses" yaml:"addresses"`
	// Name of the entity consuming the network address
	UsedBy string `json:"used_by" yaml:"used_by"`
	// Type of the entity consuming the network address
	Type string `json:"type" yaml:"type"`
	// Whether the entity comes from a network that LXD performs egress source NAT on
	NAT bool `json:"nat" yaml:"nat"`
	// Hwaddr is the MAC address of the entity consuming the network address
	Hwaddr string `json:"hwaddr" yaml:"hwaddr"`
	// Network is the name of the network the allocated address belongs to
	Network string `json:"network" yaml:"network"`
}
