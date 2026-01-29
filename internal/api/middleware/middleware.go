package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs request information
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Filter out HTTP/2 connection preface attempts
		if c.Request.Method == "PRI" {
			c.AbortWithStatus(400)
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[API] %s %s %d %v", c.Request.Method, path, status, latency)
	}
}

// Recovery recovers from panics and returns a 500 error
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[API] Panic recovered: %v", err)
				c.AbortWithStatusJSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// CORS adds CORS headers
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// ValidateChain validates the chain parameter
func ValidateChain() gin.HandlerFunc {
	return func(c *gin.Context) {
		chain := c.Param("chain")
		if chain != "btc" && chain != "ltc" {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "Invalid chain parameter. Must be 'btc' or 'ltc'",
			})
			return
		}
		c.Next()
	}
}
