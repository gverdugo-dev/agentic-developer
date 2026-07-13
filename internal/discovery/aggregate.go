package discovery

import "sort"

// This file groups the per-config-dir results into the artifact-centric
// views: every skill/plugin/marketplace across the whole scan, each with the
// list of places it lives in. This is what powers the skills, plugins and
// marketplaces views of the TUI and the `adev list` command.

// Located ties one artifact occurrence to the config dir it lives in.
type Located[T any] struct {
	ConfigDir string
	Item      T
	// Hash is the content hash of this occurrence's dir, "" when it could
	// not be hashed.
	Hash string
	// DiffCount is how many files differ between this occurrence and the
	// group's first location (the reference copy): 0 for the reference
	// itself and for unhashable occurrences.
	DiffCount int
}

// DriftState classifies the locations of one artifact group by content.
type DriftState int

// The drift states of a group.
const (
	// DriftSingle: one location, nothing to compare against.
	DriftSingle DriftState = iota
	// DriftIdentical: every location holds byte-identical content.
	DriftIdentical
	// DriftDrifted: at least one location's content differs.
	DriftDrifted
	// DriftUnknown: some location could not be hashed, so the group cannot
	// be classified.
	DriftUnknown
)

// String names the state for display and JSON output.
func (d DriftState) String() string {
	switch d {
	case DriftIdentical:
		return "identical"
	case DriftDrifted:
		return "drifted"
	case DriftUnknown:
		return "unknown"
	default:
		return "single"
	}
}

// MarshalText makes the state encode as its name in JSON.
func (d DriftState) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// classifyLocations computes each location's diff count against the group's
// first location and returns the group's drift state.
func classifyLocations[T any](locs []Located[T], files func(T) map[string]string) DriftState {
	if len(locs) <= 1 {
		return DriftSingle
	}

	reference := files(locs[0].Item)
	state := DriftIdentical
	unknown := false
	for i := range locs {
		if i > 0 {
			locs[i].DiffCount = countFileDiffs(reference, files(locs[i].Item))
		}
		if locs[i].Hash == "" {
			unknown = true
		}
		if locs[i].Hash != locs[0].Hash {
			state = DriftDrifted
		}
	}
	if unknown {
		return DriftUnknown
	}
	return state
}

// SkillGroup is one skill name and everywhere it exists.
type SkillGroup struct {
	Name        string
	Description string // first non-empty description across locations
	Drift       DriftState
	Locations   []Located[Skill]
}

// GroupSkills groups the skills of every config dir by name, sorted by name;
// each group's locations keep the scan order (user config dirs first).
func GroupSkills(dirs []ConfigDir) []SkillGroup {
	index := make(map[string]*SkillGroup)
	var order []string

	for _, dir := range dirs {
		for _, skill := range dir.Skills {
			group, ok := index[skill.Name]
			if !ok {
				group = &SkillGroup{Name: skill.Name}
				index[skill.Name] = group
				order = append(order, skill.Name)
			}
			if group.Description == "" {
				group.Description = skill.Description
			}
			group.Locations = append(group.Locations, Located[Skill]{ConfigDir: dir.Path, Item: skill, Hash: skill.Hash})
		}
	}

	sort.Strings(order)
	groups := make([]SkillGroup, 0, len(order))
	for _, name := range order {
		group := index[name]
		group.Drift = classifyLocations(group.Locations, func(s Skill) map[string]string { return s.fileHashes })
		groups = append(groups, *group)
	}
	return groups
}

// PluginGroup is one plugin identity and everywhere it is installed. The
// identity is "name@marketplace" when a marketplace is known, since the same
// plugin name can exist in two marketplaces; bare folder plugins group by
// name alone.
type PluginGroup struct {
	Key         string // name@marketplace, or just the name
	Name        string
	Marketplace string
	Version     string // first non-empty across locations
	Description string // first non-empty across locations
	Drift       DriftState
	Locations   []Located[Plugin]
}

// GroupPlugins groups the plugins of every config dir by identity, sorted by
// key.
func GroupPlugins(dirs []ConfigDir) []PluginGroup {
	index := make(map[string]*PluginGroup)
	var order []string

	for _, dir := range dirs {
		for _, plugin := range dir.Plugins {
			key := plugin.Name
			if plugin.Marketplace != "" {
				key += "@" + plugin.Marketplace
			}
			group, ok := index[key]
			if !ok {
				group = &PluginGroup{Key: key, Name: plugin.Name, Marketplace: plugin.Marketplace}
				index[key] = group
				order = append(order, key)
			}
			if group.Version == "" {
				group.Version = plugin.Version
			}
			if group.Description == "" {
				group.Description = plugin.Description
			}
			group.Locations = append(group.Locations, Located[Plugin]{ConfigDir: dir.Path, Item: plugin, Hash: plugin.Hash})
		}
	}

	sort.Strings(order)
	groups := make([]PluginGroup, 0, len(order))
	for _, key := range order {
		group := index[key]
		group.Drift = classifyLocations(group.Locations, func(p Plugin) map[string]string { return p.fileHashes })
		groups = append(groups, *group)
	}
	return groups
}

// MarketplaceGroup is one marketplace name and everywhere it is registered.
type MarketplaceGroup struct {
	Name        string
	Source      string   // first non-empty across locations
	PluginNames []string // first non-empty catalog across locations
	Drift       DriftState
	Locations   []Located[Marketplace]
}

// GroupMarketplaces groups the marketplaces of every config dir by name,
// sorted by name.
func GroupMarketplaces(dirs []ConfigDir) []MarketplaceGroup {
	index := make(map[string]*MarketplaceGroup)
	var order []string

	for _, dir := range dirs {
		for _, mkt := range dir.Marketplaces {
			group, ok := index[mkt.Name]
			if !ok {
				group = &MarketplaceGroup{Name: mkt.Name}
				index[mkt.Name] = group
				order = append(order, mkt.Name)
			}
			if group.Source == "" {
				group.Source = mkt.Source
			}
			if len(group.PluginNames) == 0 {
				group.PluginNames = mkt.PluginNames
			}
			group.Locations = append(group.Locations, Located[Marketplace]{ConfigDir: dir.Path, Item: mkt, Hash: mkt.Hash})
		}
	}

	sort.Strings(order)
	groups := make([]MarketplaceGroup, 0, len(order))
	for _, name := range order {
		group := index[name]
		group.Drift = classifyLocations(group.Locations, func(m Marketplace) map[string]string { return m.fileHashes })
		groups = append(groups, *group)
	}
	return groups
}
