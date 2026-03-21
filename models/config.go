package models

type Config struct {
	Region                   string            `json:"region"`
	ConnectionString         string            `json:"connection_string"`
	MinConnections           int               `json:"min_connections"`
	MaxOpenConnections       int               `json:"max_open_connections"`
	MaxIdleConnections       int               `json:"max_idle_connections"`
	ConnectionTimeout        int               `json:"connection_timeout"`
	QueryTimeout             int               `json:"query_timeout"`
	SessionTimeout           int               `json:"session_timeout"`
	MaxConnectionLifetime    int               `json:"max_connection_lifetime"`
	IdleTimeout              int               `json:"idle_timeout"`
	Username                 string            `json:"username"`
	Password                 string            `json:"password,omitempty"`
	SSLMode                  string            `json:"ssl_mode"`
	SSLCert                  string            `json:"ssl_cert"`
	SSLKey                   string            `json:"ssl_key"`
	SSLRootCert              string            `json:"ssl_root_cert"`
	SSLKeyPassword           string            `json:"ssl_key_password,omitempty"`
	SSLCipherSuites          string            `json:"ssl_cipher_suites"`
	SSLSecure                bool              `json:"ssl_secure"`
	SSLVersion               string            `json:"ssl_version"`
	SessionVariables         map[string]string `json:"session_variables"`
	MaxRetries               int               `json:"max_retries"`
	InitialRetryInterval     int               `json:"initial_retry_interval"`
	MaxRetryInterval         int               `json:"max_retry_interval"`
	RetryMultiplier          float64           `json:"retry_multiplier"`
	ConnectionStringTemplate string            `json:"connection_string_template"`
	Server                   string            `json:"server"`
	Servers                  []string          `json:"servers"`
	Port                     int               `json:"port"`
	Database                 string            `json:"database"`
	GracefulDegradationMode  bool              `json:"graceful_degradation_mode"`
	MaxResultRows            int               `json:"max_result_rows"`
	EvictionPolicy           string            `json:"eviction_policy"`
	EvictionBatchSize        int               `json:"eviction_batch_size"`
	EvictionGracePeriod      int               `json:"eviction_grace_period"`
	MinEvictableIdleTime     int               `json:"min_evictable_idle_time"`
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
