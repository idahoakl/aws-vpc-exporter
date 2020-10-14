package config

type Config struct {
	Subnet *SubnetConfig
}

type SubnetConfig struct {
	// fields to filter subnets
	Filter Filter

	// controls including/excluding data from info metrics
	Info SubnetInfo
}

type SubnetInfo struct {
	ExcludeAZ   bool
	ExcludeVPC  bool
	ExcludeCIDR bool
	IncludeTags []string
}

type Filter struct {
	Ids        []*string
	TagFilters []TagFilter
}

type TagFilter struct {
	Key    string
	Values []*string
}
