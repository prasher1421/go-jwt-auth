package helpers

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func CheckUserType(c *gin.Context,role string) (err error){
	userType := c.GetString("user_type")
	err = nil
	if userType != role{
		err = errors.New("Unauthorized to access this resources")
		return err
	}
	return err
}

func MatchUserTypeToUid(c *gin.Context, userId string) (err error){
	userType := c.GetString("user_type")
	uid := c.GetString("uid") //persons uid
	//user_id is id user searched for
	err=nil

	if userType == "USER" && uid != userId {
		err = errors.New("Unauthorized to access this resources")
		return err
	}

	err = CheckUserType(c,userType)
	return err
}