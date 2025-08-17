package match

import (
	"os"

	"gopkg.in/yaml.v3"
)

type MatchConfig struct {
	MatchType      int    `yaml:"match_type"`
	GameID         int    `yaml:"gameid"`
	MatchID        int32  `yaml:"matchid"`
	PlayerPerTable int    `yaml:"player_per_table"`
	InitialChips   int    `yaml:"initial_chips"`
	ScoreBase      int    `yaml:"score_base"`
	Property       string `yaml:"property"`
}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(path string) (*MatchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config MatchConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
