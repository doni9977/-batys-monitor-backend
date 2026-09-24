import re

with open("cmd/main.go", "r", encoding="utf-8") as f:
    content = f.read()

# 1. Add auth import
if "internal/auth" not in content:
    content = content.replace(
        '"github.com/danialmarat/batys-monitor-backend/internal/database"',
        '"github.com/danialmarat/batys-monitor-backend/internal/auth"\n\t"github.com/danialmarat/batys-monitor-backend/internal/database"'
    )

# 2. Add middleware and login route
auth_middleware = """	app.Use("/api", func(c *fiber.Ctx) error {
		if c.Path() == "/api/health" || c.Path() == "/api/auth/login" {
			return c.Next()
		}
		return auth.RequireJWT(c)
	})

	// Health check"""

if "auth.RequireJWT" not in content:
    content = content.replace("	// Health check", auth_middleware)

login_route = """	})
	app.Post("/api/auth/login", handlers.Login)"""

if "/api/auth/login" not in content:
    content = content.replace("	})\n\n	// Загрузка данных", login_route + "\n\n	// Загрузка данных")

with open("cmd/main.go", "w", encoding="utf-8") as f:
    f.write(content)

