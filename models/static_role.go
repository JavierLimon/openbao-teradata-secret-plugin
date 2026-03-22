package models

type StaticRole struct {
	Name               string   `json:"name"`
	Username           string   `json:"username"`
	DBName             string   `json:"db_name"`
	RotationPeriod     int      `json:"rotation_period"`
	RotationSchedule   string   `json:"rotation_schedule"`
	RotationWindow     int      `json:"rotation_window"`
	RotationStatements []string `json:"rotation_statements"`
	LastRotation       int64    `json:"last_rotation"`
	RotationCount      int      `json:"rotation_count"`
	Version            int      `json:"version"`
}

const StaticRoleVersion = 1

type StaticCredential struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	RoleName     string `json:"role_name"`
	DBName       string `json:"db_name"`
	LastRotated  int64  `json:"last_rotated"`
	NextRotation int64  `json:"next_rotation"`
}
