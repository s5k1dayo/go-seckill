package handler
import(
	"net/http"
	"github.com/gin-gonic/gin"
	"database/sql"
	//"fmt"
	"log"
	_ "github.com/go-sql-driver/mysql"
	"seckill/internal/db"
	"seckill/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func LoginHandler(c *gin.Context){
	var req struct{
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	err:=c.ShouldBindJSON(&req);if err!=nil{
		c.JSON(http.StatusBadRequest,gin.H{"error":"输入格式不正确"})
		return
	}
	var paswd string
	var name string
	var id int64
	DB:=db.GetDB();
	err=DB.QueryRow("SELECT password,username,id FROM user WHERE username=?",req.Username).Scan(&paswd,&name,&id)
	if err==sql.ErrNoRows{
		c.JSON(http.StatusBadRequest,gin.H{"error":"未找到用户"})
		return
	}else if err!=nil {
		c.JSON(http.StatusBadRequest,gin.H{"error":"用户查询失败"})
		log.Println(err.Error())
		return
	}
	err=bcrypt.CompareHashAndPassword(
		[]byte(paswd),
		[]byte(req.Password),
	)
	if(err!=nil){
		c.JSON(http.StatusBadRequest,gin.H{"error":"密码错误"})
		return
	}else{
		token,jwterr:=auth.GenerateToken(id)
		if jwterr!=nil{
			c.JSON(http.StatusBadRequest,gin.H{"error":"鉴权错误"})
			log.Println(jwterr.Error())
			return
		}
		c.JSON(http.StatusOK,gin.H{"message":"登陆成功","data":gin.H{"token":token,}})

	}
}
