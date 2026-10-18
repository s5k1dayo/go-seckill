package middleware
import(
	"seckill/internal/auth"
	"net/http"
	"github.com/gin-gonic/gin"
	"strings"
	"log"
)

func AuthMiddleware() gin.HandlerFunc{
	return func(c *gin.Context){
		token:=c.GetHeader("Authorization")
		token=strings.TrimPrefix(token,"Bearer ")
		claims,err:=auth.ParseToken(token)
		if err!=nil{
			c.JSON(http.StatusUnauthorized,gin.H{"error":"未授权"})
			log.Println(err.Error())
			c.Abort()
			return
		}
		c.Set("userid",claims.UserID)
		c.Next()
	}
}
