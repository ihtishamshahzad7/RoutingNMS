package metrics

// Standard labels keep metric cardinality predictable across vendors.
const (
	LabelOrganization = "organization_id"
	LabelDevice       = "device_id"
	LabelInterface    = "interface"
	LabelVendor       = "vendor"
	LabelDeviceType   = "device_type"
	LabelRegion       = "region"
)
