package handlers

import (
	"io"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/danialmarat/batys-monitor-backend/internal/auth"
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func mockHandlerDatabase(t *testing.T) sqlmock.Sqlmock {
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

func handlerTestToken(t *testing.T) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "logout-jti",
			Subject:   "admin",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestLogoutRevokesTokenAndCancelsActiveJobs(t *testing.T) {
	mock := mockHandlerDatabase(t)
	token := handlerTestToken(t)
	parsed, parseErr := jwt.ParseWithClaims(token, &auth.Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if parseErr != nil || !parsed.Valid {
		t.Fatalf("test token is invalid: %v", parseErr)
	}
	parsedClaims := parsed.Claims.(*auth.Claims)
	if parsedClaims.ID != "logout-jti" || parsedClaims.ExpiresAt == nil {
		t.Fatalf("test claims were not decoded: id=%q expires=%v", parsedClaims.ID, parsedClaims.ExpiresAt)
	}
	mock.ExpectQuery(`SELECT .*revoked_tokens.*`).
		WithArgs("logout-jti", 1).WillReturnRows(sqlmock.NewRows([]string{"id", "jti", "expires_at", "created_at"}).
		AddRow(1, "logout-jti", time.Now().Add(time.Hour), time.Now()))
	mock.ExpectQuery(`SELECT .*"risk_jobs".*username.*status`).
		WithArgs("admin", models.RiskJobStatusQueued, models.RiskJobStatusRunning).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "username"}))

	app := fiber.New()
	app.Post("/logout", func(c *fiber.Ctx) error {
		c.Locals("admin_username", "admin")
		return Logout(c)
	})
	request := httptest.NewRequest("POST", "/logout", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status=%d, want %d, body=%s, mock=%v", response.StatusCode, fiber.StatusOK, body, mock.ExpectationsWereMet())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelRiskJobRejectsForeignJob(t *testing.T) {
	mock := mockHandlerDatabase(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "risk_jobs" WHERE "risk_jobs"."id" = $1 ORDER BY "risk_jobs"."id" LIMIT $2`)).
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "username"}).AddRow(7, models.RiskJobStatusRunning, "another-user"))

	app := fiber.New()
	app.Post("/jobs/:id/cancel", func(c *fiber.Ctx) error {
		c.Locals("admin_username", "admin")
		return CancelRiskJob(c)
	})
	response, err := app.Test(httptest.NewRequest("POST", "/jobs/7/cancel", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelRiskJobRejectsCompletedJob(t *testing.T) {
	mock := mockHandlerDatabase(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "risk_jobs" WHERE "risk_jobs"."id" = $1 ORDER BY "risk_jobs"."id" LIMIT $2`)).
		WithArgs(8, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "username"}).AddRow(8, models.RiskJobStatusDone, "admin"))

	app := fiber.New()
	app.Post("/jobs/:id/cancel", func(c *fiber.Ctx) error {
		c.Locals("admin_username", "admin")
		return CancelRiskJob(c)
	})
	response, err := app.Test(httptest.NewRequest("POST", "/jobs/8/cancel", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusConflict {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusConflict)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
