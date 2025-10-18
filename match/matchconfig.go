package match

import (
	"os"

	"gopkg.in/yaml.v3"
)

type MatchConfig struct {
	GameType       string `yaml:"game_type"`
	MatchID        int32  `yaml:"matchid"`
	Name           string `yaml:"name"`
	PlayerPerTable int32  `yaml:"player_per_table"`
	Diamond        int32  `yaml:"diamond"`
	InitialChips   int64  `yaml:"initial_chips"`
	ScoreBase      int64  `yaml:"score_base"`
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
