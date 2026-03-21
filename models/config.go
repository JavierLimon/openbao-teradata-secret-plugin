package models

type Config struct {
	ConnectionString   string `json:"connection_string"`
	MaxOpenConnections int    `json:"max_open_connections"`
	MaxIdleConnections int    `json:"max_idle_connections"`
	ConnectionTimeout  int    `json:"connection_timeout"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
}

type Role struct {
	Name                string `json:"name"`
	DBUser              string `json:"db_user"`
	UsernamePrefix      string `json:"username_prefix"`
	DBPassword          string `json:"db_password,omitempty"`
	DefaultTTL          int    `json:"default_ttl"`
	MaxTTL              int    `json:"max_ttl"`
	RenewalPeriod       int    `json:"renewal_period"`
	StatementTemplate   string `json:"statement_template"`
	CreationStatement   string `json:"creation_statement"`
	RevocationStatement string `json:"revocation_statement"`
	RollbackStatement   string `json:"rollback_statement"`
	RenewalStatement    string `json:"renewal_statement"`
	DefaultDatabase     string `json:"default_database"`
	PermSpace           int64  `json:"perm_space"`
	SpoolSpace          int64  `json:"spool_space"`
	Account             string `json:"account"`
	Fallback            bool   `json:"fallback"`
	BatchSize           int    `json:"batch_size"`
}

type Statement struct {
	Name                string `json:"name"`
	CreationStatement   string `json:"creation_statement"`
	RevocationStatement string `json:"revocation_statement"`
	RollbackStatement   string `json:"rollback_statement"`
	RenewalStatement    string `json:"renewal_statement"`
}
