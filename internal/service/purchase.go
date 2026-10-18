package service
import(
	"seckill/internal/db"
	"errors"
	"log"
)

func Purchase(UserID int64,ProductID int64)(int64,error){
	DB:=db.GetDB()
	tx,err:=DB.Begin()
	if err!=nil{
		return 0,err
	}
	defer tx.Rollback()
	var stock int64
	res,err:=tx.Exec("UPDATE product SET stock=stock -1 WHERE id = ? AND stock > 0",ProductID)
	if err!=nil{
		log.Println(err.Error())
		return 0,errors.New("数据获取失败")
	}
	if n,_:=res.RowsAffected();n==0{
		return 0,errors.New("库存不足")
	}
	_,err=tx.Exec("INSERT INTO orders (user_id,product_id) VALUES (?,?)",UserID,ProductID)
	if db.IsDuplicateKey(err){
		return 0,errors.New("请勿重复购买")
	}else if err!=nil{
		log.Println(err.Error())
		return 0,errors.New("数据获取失败")
	}
	err=tx.QueryRow("SELECT stock FROM product WHERE id=?",ProductID).Scan(&stock)
	if err!=nil{
		log.Println(err.Error())
		return 0,errors.New("数据获取失败")
	}
	if err=tx.Commit(); err!=nil{
		log.Println(err.Error())
		return 0,errors.New("数据处理失败")
	}
	return stock,nil

}
