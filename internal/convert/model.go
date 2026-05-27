package convert

type Node struct {
	Name     string
	Protocol string
	Server   string
	Port     int
	Params   map[string]string
	Tags     []string
	Pinned   bool
}

type MergeOptions struct {
	Mode string
}

type SurgeConfig struct {
	CustomRules         []string
	CustomGroups        []string
	RuleMergeMode       string
	ManagedConfigHeader string
}
