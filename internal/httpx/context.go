package httpx

import (
	"strconv"

	platformjwt "example-company/internal/platform/jwt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ClaimsKey = "auth_claims"

func Claims(c *gin.Context) *platformjwt.Claims {
	value, ok := c.Get(ClaimsKey)
	if !ok {
		return nil
	}
	claims, _ := value.(*platformjwt.Claims)
	return claims
}

func UUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Param(name))
}

func Pagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return
}
