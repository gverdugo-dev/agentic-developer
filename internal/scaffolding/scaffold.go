package scaffolding

import (
	_ "embed"
	"encoding/json"
	"log/slog"
)

//go:embed structures.json
var structuresConfig []byte

type Config struct {
	Structures struct {
		Skill             []string `json:"skill"`
		Plugin            []string `json:"plugin"`
		PluginMarketplace []string `json:"pluginMarketplace"`
	} `json:"structures"`
}

func LoadConfig() Config {

	var data Config

	if err := json.Unmarshal(structuresConfig, &data); err != nil {
		slog.Error("error loading json data", "err", err)
	}

	return data
}
