package config

type AppCfg struct {
    Log logCfg `yaml:"Log" json:"Log"`
    Bgp bgpCfg `yaml:"Bgp" json:"Bgp"`
    Dns dnsCfg `yaml:"Dns" json:"Dns"`
}

