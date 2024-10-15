package config

type AppCfg struct {
    Log logCfg `yaml:"Log" json:"Log"`
    Dns dnsCfg `yaml:"Dns" json:"Dns"`
}

