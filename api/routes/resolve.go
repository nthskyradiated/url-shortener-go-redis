package routes

import (
	"github.com/nthskyradiated/url-shortener-go-redis/db"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")

	r := db.CreateClient(0)
	defer r.Close()

	value, err := r.Get(db.Ctx, url).Result()
	if err == redis.Nil{
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "short url not found in db",
		})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "cannot connect to db",
		})
	}
	rInr := db.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(db.Ctx, "counter")

	return c.Redirect(value, 301)
}