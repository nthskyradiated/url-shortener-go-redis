package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/nthskyradiated/url-shortener-go-redis/db"
	"github.com/nthskyradiated/url-shortener-go-redis/helpers"
	"github.com/asaskevich/govalidator"
)

type request struct {
	URL string				`json:"url"`
	CustomShort string 		`json:"short"`
	Expiry time.Duration 	`json:"expiry"`
}

type response struct {
	URL string				`json:"url"`
	CustomShort string		`json:"short"`
	Expiry time.Duration	`json:"expiry"`
	XRateRemaining int		`json:"rateLimit"`
	XRateLimitReset time.Duration `json:"rateLimitReset"`

}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)
	if err := c.BodyParser(&body); err !=nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cannot parse JSON",
		})
	}
	//implement rate limit
	r2 := db.CreateClient(1)
	defer r2.Close()
	val, err := r2.Get(db.Ctx, c.IP()).Result()

	if err == redis.Nil{
		_ = r2.Set(db.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err()
	} else {
		val, _ = r2.Get(db.Ctx, c.IP()).Result()
		valInt, _ := strconv.Atoi(val)
		if valInt <= 0 {
			limit, _ := r2.TTL(db.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "rate limit exceeded",
				"rate_limit_reset": limit / time.Nanosecond / time.Minute, 
			})
		}
	}



	//check input URL
	if !govalidator.IsURL(body.URL){
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid url",
		})
	}
	//check domain error

	if !helpers.RemoveDomainError(body.URL){
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "☠️",
		})
	}
	//enforce https
	body.URL = helpers.EnforceHTTP(body.URL)

	var id string

	if body.CustomShort == ""{
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	r:= db.CreateClient(0)
	defer r.Close()

	val, _ = r.Get(db.Ctx, id).Result()
	if val != ""{
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "URL short already in use.",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = r.Set(db.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unable to connect to server",
		})
	}

	res := response{
		URL: body.URL,
		CustomShort: "",
		Expiry: body.Expiry,
		XRateRemaining: 10,
		XRateLimitReset: 30,
	}

	r2.Decr(db.Ctx, c.IP())
	val, _ = r2.Get(db.Ctx, c.IP()).Result()
	res.XRateRemaining, _ = strconv.Atoi(val)

	ttl, _ := r2.TTL(db.Ctx, c.IP()).Result()
	res.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	res.CustomShort = os.Getenv("DOMAIN") + "/" + id
	return c.Status(fiber.StatusOK).JSON(res)
}