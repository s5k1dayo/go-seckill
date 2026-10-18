package auth
import(
	"errors"
	"time"
	"log"
	"github.com/golang-jwt/jwt/v5"
    "os"
)

var secretKey []byte
func Jwt_init(){
	secretkey:=[]byte(os.Getenv("JWT_SECRET_KEY"))
	if len(secretkey)==0 {
		log.Fatal("jwt_secret_key不存在")
	}
	secretKey=secretkey
}
//密钥，需要进环境变量
//定义claim结构体
type Claims struct{
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64) (string,error){
	now:=time.Now()
	claims:=Claims{
		userID,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt: jwt.NewNumericDate(now),
			Issuer: "seckill",
		},
	}
	token:=jwt.NewWithClaims(jwt.SigningMethodHS256,claims)
	res,err:= token.SignedString(secretKey)//signedstring函数返回是（string，error）
	if err!=nil{
		log.Printf("生成token失败: user_id=%v : %v",userID ,err)
		return "",errors.New("生成凭证失败")
	}
	return res,nil;
}

func ParseToken(tokenStr string) (*Claims,error){
	token,err:=jwt.ParseWithClaims(tokenStr,&Claims{},func(t *jwt.Token)(any,error){
		if _,ok:=t.Method.(*jwt.SigningMethodHMAC);!ok{
			return nil,errors.New("unexpected signing method")
		}
		return secretKey,nil
	})
	if err!=nil{
		return nil,err
	}
	if claims,ok:=token.Claims.(*Claims);ok && token.Valid{
		return claims,nil
	}
	return nil,errors.New("invalid token")
}
