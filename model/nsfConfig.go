package model

type NsfConfig struct {
	Nsf    NsfIE    `yaml:"nsf" validate:"required"`
	Logger LoggerIE `yaml:"logger" validate:"required"`
}

type NsfIE struct {
	SbiAddress string `yaml:"sbi_address" validate:"required"`
	SbiPort    int    `yaml:"sbi_port" validate:"required"`

	DbType  string    `yaml:"db_type" validate:"required"`
	MongoDB MongoDBIE `yaml:"mongodb"`
}

type MongoDBIE struct {
	URI  string `yaml:"uri"`
	Name string `yaml:"name"`
}

type LoggerIE struct {
	Level string `yaml:"level" validate:"required"`
}
