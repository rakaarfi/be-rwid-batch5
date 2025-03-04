package main

import (
	"io"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
)
 
func main() {
    app := fiber.New(fiber.Config{
        Prefork: true,
    })
 
    // first endpoint
    app.Get("/", func (c *fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    // second endpoint
    resp, err := http.Get("https://api.rss2json.com/v1/api.json?rss_url=https://medium.com/feed/@rakaarfi")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }

    app.Get("/api-medium-rss", func (c *fiber.Ctx) error {
        return c.SendString(string(body))
    })

    // third endpoint
    book := map[string]string{
        "title": "The Go Programming Language",
        "author": "Robert Griesemer",
        "year": "2009",
    }
    app.Get("/book", func (c *fiber.Ctx) error {
        return c.JSON(book)
    })
 
    log.Fatal(app.Listen(":3003"))
}
