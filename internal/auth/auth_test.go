package auth

import (
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func mockDatabase(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = gormDB
	t.Cleanup(func() { _ = sqlDB.Close() })
	return mock
}

func testToken(t *testing.T, expiresAt time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			Subject:   "admin",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}).SignedString([]byte(jwtSecret()))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestLogin(t *testing.T) {
	mock := mockDatabase(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "admins" WHERE username = $1 ORDER BY "admins"."id" LIMIT $2`)).
		WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "created_at"}).
			AddRow(1, "admin", string(hash), time.Now()))

	token, err := Login("admin", "password")
	if err != nil || token == "" {
		t.Fatalf("Login() token=%q err=%v", token, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireJWTRejectsRevokedToken(t *testing.T) {
	mock := mockDatabase(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "revoked_tokens" WHERE jti = $1 ORDER BY "revoked_tokens"."id" LIMIT $2`)).
		WithArgs("test-jti", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "jti", "expires_at", "created_at"}).
			AddRow(1, "test-jti", time.Now().Add(time.Hour), time.Now()))

	app := fiber.New()
	app.Use(RequireJWT)
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	request := httptest.NewRequest("GET", "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+testToken(t, time.Now().Add(time.Hour)))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestRequireJWTRejectsExpiredToken(t *testing.T) {
	app := fiber.New()
	app.Use(RequireJWT)
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	request := httptest.NewRequest("GET", "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+testToken(t, time.Now().Add(-time.Hour)))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestRequireJWTRejectsMissingAuthorization(t *testing.T) {
	app := fiber.New()
	app.Use(RequireJWT)
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	response, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusUnauthorized)
	}
}
