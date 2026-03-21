package models

type Config struct {
	Region                string            `json:"region"`
	ConnectionString      string            `json:"connection_string"`
	MinConnections        int               `json:"min_connections"`
	MaxOpenConnections    int               `json:"max_open_connections"`
	MaxIdleConnections    int               `json:"max_idle_connections"`
	ConnectionTimeout     int               `json:"connection_timeout"`
	QueryTimeout          int               `json:"query_timeout"`
	MaxConnectionLifetime int               `json:"max_connection_lifetime"`
	IdleTimeout           int               `json:"idle_timeout"`
	Username              string            `json:"username"`
	Password              string            `json:"password,omitempty"`
	SSLMode               string            `json:"ssl_mode"`
	SSLCert               string            `json:"ssl_cert"`
	SSLKey                string            `json:"ssl_key"`
	SSLRootCert           string            `json:"ssl_root_cert"`
	SSLKeyPassword        string            `json:"ssl_key_password,omitempty"`
	SSLCipherSuites       string            `json:"ssl_cipher_suites"`
	SSLSecure             bool              `json:"ssl_secure"`
	SSLVersion            string            `json:"ssl_version"`
	SessionVariables      map[string]string `json:"session_variables"`
}

type Role struct {
	Name                string            `json:"name"`
	Version             int               `json:"version"`
	DBUser              string            `json:"db_user"`
	UsernamePrefix      string            `json:"username_prefix"`
	DBPassword          string            `json:"db_password,omitempty"`
	DefaultTTL          int               `json:"default_ttl"`
	MaxTTL              int               `json:"max_ttl"`
	RenewalPeriod       int               `json:"renewal_period"`
	StatementTemplate   string            `json:"statement_template"`
	CreationStatement   string            `json:"creation_statement"`
	RevocationStatement string            `json:"revocation_statement"`
	RollbackStatement   string            `json:"rollback_statement"`
	RenewalStatement    string            `json:"renewal_statement"`
	DefaultDatabase     string            `json:"default_database"`
	PermSpace           int64             `json:"perm_space"`
	SpoolSpace          int64             `json:"spool_space"`
	Account             string            `json:"account"`
	Fallback            bool              `json:"fallback"`
	BatchSize           int               `json:"batch_size"`
	MaxCredentials      int               `json:"max_credentials"`
	SessionVariables    map[string]string `json:"session_variables"`
}

const RoleVersion = 2

type Statement struct {
	Name                string `json:"name"`
	CreationStatement   string `json:"creation_statement"`
	RevocationStatement string `json:"revocation_statement"`
	RollbackStatement   string `json:"rollback_statement"`
	RenewalStatement    string `json:"renewal_statement"`
}
