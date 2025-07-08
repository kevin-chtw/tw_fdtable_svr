package match

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type MatchConfig struct {
	MaxPlayers   int           `yaml:"player_per_table"`
	Timeout      time.Duration `yaml:"timeout"`
	MinPlayers   int           `yaml:"min_players"`
	MatchType    string        `yaml:"match_type"`
	GameID       int           `yaml:"gameid"`
	InitialChips int           `yaml:"initial_chips"`
	ScoreBase    int           `yaml:"score_base"`
	GameConfig   GameRules     `yaml:"game_config"`
}

type GameRules struct {
	GameName string   `yaml:"game_name"`
	Rules    []string `yaml:"rules"`
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

	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.MinPlayers == 0 {
		config.MinPlayers = 2
	}

	return &config, nil
}
