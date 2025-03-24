package middleware

import (
	"belajar-go/pkg/logger"
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func ErrorHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Recovered from panic: %v", r)
				stack := string(debug.Stack())
				logger.ErrorLogger.Error(errMsg, zap.String("stack", stack))
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": errMsg,
				})
			}
		}()
		// Logging request masuk
		logger.RequestLogger.Info("Incoming request",
			zap.String("method", c.Method()),
			zap.String("url", c.OriginalURL()),
		)
		return c.Next()
	}
}
