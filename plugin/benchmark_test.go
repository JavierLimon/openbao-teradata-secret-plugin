package teradata

import (
	"testing"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
)

func BenchmarkGeneratePassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generatePassword()
	}
}

func BenchmarkGeneratePasswordParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			generatePassword()
		}
	})
}

func BenchmarkGenerateUsername(b *testing.B) {
	prefixes := []string{"", "vault", "myapp", "admin", "user", "db"}

	for i := 0; i < b.N; i++ {
		prefix := prefixes[i%len(prefixes)]
		generateUsername(prefix)
	}
}

func BenchmarkGenerateUsernameParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			prefix := []string{"", "vault", "myapp"}[counter%3]
			generateUsername(prefix)
			counter++
		}
	})
}

func BenchmarkEnsurePasswordRequirements(b *testing.B) {
	passwords := []string{
		"abcdefghijklmnop",
		"ABCDEFGHIJKLMNOP",
		"0123456789012345",
		"!@#$%^&*()_+-=[]",
		"aB1!aB1!aB1!aB1!",
		"abcdefghijklmnopqrstuvwxyzABCD",
	}

	for i := 0; i < b.N; i++ {
		ensurePasswordRequirements(passwords[i%len(passwords)])
	}
}

func BenchmarkEnsurePasswordRequirementsParallel(b *testing.B) {
	passwords := []string{
		"abcdefghijklmnop",
		"ABCDEFGHIJKLMNOP",
		"0123456789012345",
		"!@#$%^&*()_+-=[]",
	}

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			ensurePasswordRequirements(passwords[counter%len(passwords)])
			counter++
		}
	})
}

func BenchmarkBuildTeradataCreateUserSQL(b *testing.B) {
	role := &models.Role{
		DefaultDatabase: "mydb",
		PermSpace:       1000000,
		SpoolSpace:      5000000,
		Account:         "'$M1'",
		Fallback:        true,
	}

	usernames := []string{
		"vault_a1b2c3d4e5f6g7h8",
		"myapp_1234567890abcdef",
		"admin_fedcba0987654321",
	}

	passwords := []string{
		"aB1!cD2@eF3#gH4$iJ5%",
		"xY9!zW8@yV7@uT6#sR5$",
		"pQ1!rS2@tU3@vW4#xY5%",
	}

	for i := 0; i < b.N; i++ {
		username := usernames[i%len(usernames)]
		password := passwords[i%len(passwords)]
		buildTeradataCreateUserSQL(role, username, password)
	}
}

func BenchmarkBuildTeradataCreateUserSQLParallel(b *testing.B) {
	role := &models.Role{
		DefaultDatabase: "mydb",
		PermSpace:       1000000,
		SpoolSpace:      5000000,
		Account:         "'$M1'",
		Fallback:        true,
	}

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			username := "vault_a1b2c3d4e5f6g7h8"
			password := "aB1!cD2@eF3#gH4$iJ5%"
			buildTeradataCreateUserSQL(role, username, password)
			counter++
		}
	})
}

func BenchmarkGeneratePasswordUniqueness(b *testing.B) {
	b.ReportAllocs()

	usernames := make(map[string]int, b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		password := generatePassword()
		usernames[password]++
	}

	duplicates := 0
	for _, count := range usernames {
		if count > 1 {
			duplicates++
		}
	}

	if duplicates > 0 {
		b.Errorf("found %d duplicate passwords", duplicates)
	}
}

func BenchmarkGenerateUsernameUniqueness(b *testing.B) {
	b.ReportAllocs()

	usernames := make(map[string]int, b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		username := generateUsername("vault")
		usernames[username]++
	}

	duplicates := 0
	for _, count := range usernames {
		if count > 1 {
			duplicates++
		}
	}

	if duplicates > 0 {
		b.Errorf("found %d duplicate usernames", duplicates)
	}
}

func BenchmarkGenerateUsernameHexSuffix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		username := generateUsername("test")

		parts := splitUsername(username)
		if len(parts) != 2 {
			continue
		}

		suffix := parts[1]
		for _, c := range suffix {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				break
			}
		}
	}
}

func splitUsername(username string) []string {
	var parts []string
	var current []byte

	for i := 0; i < len(username); i++ {
		if username[i] == '_' {
			if current != nil {
				parts = append(parts, string(current))
				current = nil
			}
		} else {
			current = append(current, username[i])
		}
	}

	if current != nil {
		parts = append(parts, string(current))
	}

	return parts
}
