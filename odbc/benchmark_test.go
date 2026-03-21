package odbc

import (
	"strings"
	"testing"
)

func BenchmarkValidateUsername(b *testing.B) {
	usernames := []string{
		"validuser",
		"user123",
		"myapp_a1b2c3d4e5f6g7h8",
		"admin_fedcba0987654321",
		"vault_1234567890abcdef",
		"testuser_ABCDEF123456",
	}

	for i := 0; i < b.N; i++ {
		ValidateUsername(usernames[i%len(usernames)])
	}
}

func BenchmarkValidateUsernameParallel(b *testing.B) {
	usernames := []string{
		"validuser",
		"user123",
		"myapp_a1b2c3d4e5f6g7h8",
		"admin_fedcba0987654321",
	}

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			ValidateUsername(usernames[counter%len(usernames)])
			counter++
		}
	})
}

func BenchmarkValidateUsernameEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateUsername("")
	}
}

func BenchmarkValidateUsernameTooLong(b *testing.B) {
	longUsername := strings.Repeat("a", 35)

	for i := 0; i < b.N; i++ {
		ValidateUsername(longUsername)
	}
}

func BenchmarkValidateUsernameSQLInjection(b *testing.B) {
	maliciousUsernames := []string{
		"user'; DROP TABLE users;--",
		"admin'--",
		"test' OR '1'='1",
		"user/*comment*/",
		"test;DELETE FROM users",
	}

	for i := 0; i < b.N; i++ {
		ValidateUsername(maliciousUsernames[i%len(maliciousUsernames)])
	}
}

func BenchmarkValidateUsernameSQLKeywords(b *testing.B) {
	keywordUsernames := []string{
		"SELECT_user",
		"adminDROP",
		"userWHERE",
		"testUNION",
		"adminGRANT",
	}

	for i := 0; i < b.N; i++ {
		ValidateUsername(keywordUsernames[i%len(keywordUsernames)])
	}
}

func BenchmarkValidateUsernameValidChars(b *testing.B) {
	validUsernames := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"0123456789",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_$",
	}

	for i := 0; i < b.N; i++ {
		ValidateUsername(validUsernames[i%len(validUsernames)])
	}
}

func BenchmarkParseSQLStatements(b *testing.B) {
	sqlStatements := `
		GRANT SELECT ON database1 TO user1;
		GRANT INSERT ON database1 TO user1;
		GRANT UPDATE ON database1 TO user1;
		GRANT DELETE ON database1 TO user1;
		GRANT SELECT ON database2 TO user2;
	`

	for i := 0; i < b.N; i++ {
		parseSQLStatements(sqlStatements)
	}
}

func BenchmarkParseSQLStatementsParallel(b *testing.B) {
	sqlStatements := `
		GRANT SELECT ON database1 TO user1;
		GRANT INSERT ON database1 TO user1;
		GRANT UPDATE ON database1 TO user1;
		GRANT DELETE ON database1 TO user1;
	`

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			parseSQLStatements(sqlStatements)
		}
	})
}

func BenchmarkNormalizeGrantStatement(b *testing.B) {
	statements := []string{
		"GRANT SELECT ON database TO user",
		"  GRANT INSERT ON database TO user  ",
		"grant update on database to user",
		"GRANT DELETE ON database TO user",
	}

	for i := 0; i < b.N; i++ {
		normalizeGrantStatement(statements[i%len(statements)])
	}
}

func BenchmarkNormalizeGrantStatementNonGrant(b *testing.B) {
	statements := []string{
		"SELECT * FROM users",
		"DROP USER test",
		"CREATE TABLE test",
		"INVALID STATEMENT",
	}

	for i := 0; i < b.N; i++ {
		normalizeGrantStatement(statements[i%len(statements)])
	}
}
