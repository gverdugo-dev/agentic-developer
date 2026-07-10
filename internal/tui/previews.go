package tui

import (
	"agentic-developer/internal/discovery"
	"agentic-developer/internal/scaffolding"
	"fmt"
	"strings"
)

// This file builds the preview/detail content of every component the TUI can
// select: config dirs, skills, plugins and marketplaces, both as a single
// occurrence and as a cross-path group. The same content feeds the right
// panel preview and the full-screen detail page.

// configDirPreview summarizes one config dir: its harness, full path and the
// artifacts it holds.
func configDirPreview(dir discovery.ConfigDir) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(scaffolding.AIHarnesses[dir.Harness]) + "\n")
	b.WriteString(itemMutedStyle.Render("("+abbreviateHome(dir.Path)+")") + "\n\n")

	skills := make([]string, 0, len(dir.Skills))
	for _, s := range dir.Skills {
		skills = append(skills, s.Name)
	}
	writeArtifactSection(&b, "skills", skills)
	writeArtifactSection(&b, "plugins", pluginLabels(dir.Plugins))
	writeArtifactSection(&b, "marketplaces", marketplaceLabels(dir.Marketplaces))
	return b.String()
}

// skillPreview details one skill occurrence.
func skillPreview(s discovery.Skill) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(s.Name) + "\n")
	b.WriteString(itemMutedStyle.Render("("+abbreviateHome(s.Path)+")") + "\n")
	writeHashLine(&b, s.Hash)
	b.WriteString("\n" + descriptionOr(s.Description))
	return b.String()
}

// skillGroupPreview details one skill across every place it exists.
func skillGroupPreview(g discovery.SkillGroup) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(g.Name) + "\n\n")
	b.WriteString(descriptionOr(g.Description) + "\n\n")
	writeDriftSummary(&b, g.Drift)
	writeLocations(&b, len(g.Locations), func(i int) string {
		loc := g.Locations[i]
		return locationLabel(abbreviateHome(loc.ConfigDir), loc.Hash, loc.DiffCount)
	})
	return b.String()
}

// pluginPreview details one plugin occurrence.
func pluginPreview(p discovery.Plugin) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(p.Name))
	if p.Marketplace != "" {
		b.WriteString(itemMutedStyle.Render("@" + p.Marketplace))
	}
	b.WriteString("\n")
	if p.Version != "" {
		b.WriteString(itemMutedStyle.Render("version ") + p.Version + "\n")
	}
	if p.Marketplace != "" {
		b.WriteString(itemMutedStyle.Render("state   ") + enabledLabel(p.Enabled) + "\n")
	}
	if p.Path != "" {
		b.WriteString(itemMutedStyle.Render("("+abbreviateHome(p.Path)+")") + "\n")
	}
	writeHashLine(&b, p.Hash)
	b.WriteString("\n" + descriptionOr(p.Description))
	return b.String()
}

// pluginGroupPreview details one plugin identity across every place it is
// installed, with the enabled state per location.
func pluginGroupPreview(g discovery.PluginGroup) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(g.Name))
	if g.Marketplace != "" {
		b.WriteString(itemMutedStyle.Render("@" + g.Marketplace))
	}
	b.WriteString("\n")
	if g.Version != "" {
		b.WriteString(itemMutedStyle.Render("version ") + g.Version + "\n")
	}
	b.WriteString("\n" + descriptionOr(g.Description) + "\n\n")
	writeDriftSummary(&b, g.Drift)
	writeLocations(&b, len(g.Locations), func(i int) string {
		loc := g.Locations[i]
		label := abbreviateHome(loc.ConfigDir)
		if loc.Item.Marketplace != "" {
			label += " " + enabledLabel(loc.Item.Enabled)
		}
		return locationLabel(label, loc.Hash, loc.DiffCount)
	})
	return b.String()
}

// marketplacePreview details one marketplace occurrence.
func marketplacePreview(mkt discovery.Marketplace) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(mkt.Name) + "\n")
	if mkt.Source != "" {
		b.WriteString(itemMutedStyle.Render("source ") + mkt.Source + "\n")
	}
	if mkt.Path != "" {
		b.WriteString(itemMutedStyle.Render("("+abbreviateHome(mkt.Path)+")") + "\n")
	}
	writeHashLine(&b, mkt.Hash)
	b.WriteString("\n")
	writeCatalog(&b, mkt.PluginNames)
	return b.String()
}

// marketplaceGroupPreview details one marketplace across every place it is
// registered.
func marketplaceGroupPreview(g discovery.MarketplaceGroup) string {
	var b strings.Builder
	b.WriteString(itemSelectedStyle.Render(g.Name) + "\n")
	if g.Source != "" {
		b.WriteString(itemMutedStyle.Render("source ") + g.Source + "\n")
	}
	b.WriteString("\n")
	writeCatalog(&b, g.PluginNames)
	b.WriteString("\n")
	writeDriftSummary(&b, g.Drift)
	writeLocations(&b, len(g.Locations), func(i int) string {
		loc := g.Locations[i]
		return locationLabel(abbreviateHome(loc.ConfigDir), loc.Hash, loc.DiffCount)
	})
	return b.String()
}

// writeLocations writes the "lives in N location(s):" block.
func writeLocations(b *strings.Builder, n int, label func(int) string) {
	b.WriteString(fmt.Sprintf("lives in %d location(s):\n", n))
	for i := 0; i < n; i++ {
		b.WriteString(itemMutedStyle.Render("  - ") + label(i) + "\n")
	}
}

// writeHashLine writes the content-hash line of a single artifact preview,
// nothing when the artifact could not be hashed.
func writeHashLine(b *strings.Builder, hash string) {
	if hash == "" {
		return
	}
	b.WriteString(itemMutedStyle.Render("hash ") + discovery.ShortHash(hash) + "\n")
}

// writeDriftSummary phrases a duplicated group's content comparison at the
// top of its locations block; single groups need none.
func writeDriftSummary(b *strings.Builder, d discovery.DriftState) {
	var line string
	switch d {
	case discovery.DriftIdentical:
		line = itemSelectedStyle.Render("= ") + "identical in every location"
	case discovery.DriftDrifted:
		line = errorTextStyle.Render("≠ ") + "content drifted between locations"
	case discovery.DriftUnknown:
		line = itemMutedStyle.Render("content state unknown")
	default:
		return
	}
	b.WriteString(line + "\n\n")
}

// driftBadge marks a duplicated group's content state on its list row: "="
// when every copy is identical, "≠" when they drifted. Single and unknown
// groups get no badge.
func driftBadge(d discovery.DriftState) string {
	switch d {
	case discovery.DriftIdentical:
		return itemSelectedStyle.Render("=")
	case discovery.DriftDrifted:
		return errorTextStyle.Render("≠")
	}
	return ""
}

// locationLabel appends a location's hash state to its label: the short
// content hash, plus how many files differ from the group's first location
// when any do.
func locationLabel(label, hash string, diffCount int) string {
	if hash == "" {
		return label
	}
	label += " " + itemMutedStyle.Render(discovery.ShortHash(hash))
	if diffCount > 0 {
		label += " " + errorTextStyle.Render(fmt.Sprintf("≠ %d file(s) differ", diffCount))
	}
	return label
}

// writeCatalog writes the "offers N plugin(s):" block of a marketplace.
func writeCatalog(b *strings.Builder, names []string) {
	if len(names) == 0 {
		b.WriteString(itemMutedStyle.Render("no plugin catalog found") + "\n")
		return
	}
	b.WriteString(fmt.Sprintf("offers %d plugin(s):\n", len(names)))
	for _, name := range names {
		b.WriteString(itemMutedStyle.Render("  - ") + name + "\n")
	}
}

// descriptionOr returns the description, or a muted placeholder.
func descriptionOr(desc string) string {
	if desc == "" {
		return itemMutedStyle.Render("no description")
	}
	return desc
}

// enabledLabel renders the on/off state of a registry plugin.
func enabledLabel(enabled bool) string {
	if enabled {
		return itemSelectedStyle.Render("on")
	}
	return itemMutedStyle.Render("off")
}

// writeArtifactSection writes one "skills (2):" block with its item list, or
// just the zero-count heading when the category is empty.
func writeArtifactSection(b *strings.Builder, name string, items []string) {
	b.WriteString(fmt.Sprintf("%s (%d)", name, len(items)))
	if len(items) == 0 {
		b.WriteString("\n")
		return
	}
	b.WriteString(":\n")
	for _, item := range items {
		b.WriteString(itemMutedStyle.Render("  - ") + item + "\n")
	}
}

// pluginLabels formats each plugin for a config dir section. Registry-backed
// plugins (they carry a marketplace) show name@marketplace, version and
// their enabled state, like the harness's own /plugins screen; folder-based
// ones are just the name.
func pluginLabels(plugins []discovery.Plugin) []string {
	labels := make([]string, 0, len(plugins))
	for _, p := range plugins {
		label := p.Name
		if p.Marketplace != "" {
			label += itemMutedStyle.Render("@" + p.Marketplace)
			if p.Version != "" {
				label += " " + itemMutedStyle.Render(p.Version)
			}
			label += " " + enabledLabel(p.Enabled)
		}
		labels = append(labels, label)
	}
	return labels
}

// marketplaceLabels formats each marketplace for a config dir section: the
// name plus its source when the registry provides one.
func marketplaceLabels(marketplaces []discovery.Marketplace) []string {
	labels := make([]string, 0, len(marketplaces))
	for _, mkt := range marketplaces {
		label := mkt.Name
		if mkt.Source != "" {
			label += " " + itemMutedStyle.Render("("+mkt.Source+")")
		}
		labels = append(labels, label)
	}
	return labels
}
