package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/prasher1421/go-jwt/database"
	"github.com/prasher1421/go-jwt/helpers"
	"github.com/prasher1421/go-jwt/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var userCollection *mongo.Collection = database.OpenCollection(database.Client,"user")
var validate = validator.New()

func HashPassword(pass string ) string{
	bytes,err := bcrypt.GenerateFromPassword([]byte(pass),14)
	if err!=nil {
		log.Panic(err)
	}
	return string(bytes)
}

func VerifyPassword(userPass string, providedPass string)(bool,string) {
	err := bcrypt.CompareHashAndPassword([]byte(providedPass),[]byte(userPass))
	check := true
	msg := ""

	if err!=nil {
		msg = fmt.Sprintf("Password is incorrect")
		check = false
	}

	return check,msg
}

func Signup() gin.HandlerFunc{
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(),100*time.Second)
		var user models.User

		if err := c.BindJSON(&user); err!=nil {
			c.JSON(http.StatusBadRequest,gin.H{"error":err.Error()})
		}

		validationErr := validate.Struct(user)
		if validationErr!=nil {
			c.JSON(http.StatusBadRequest, gin.H{"error":validationErr.Error()})
			defer cancel()
			return
		}

		count,err := userCollection.CountDocuments(ctx,bson.M{"email":user.Email})
		defer cancel()
		if err!=nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError,gin.H{"error":"error occurred while checking the email"})
		}

		password := HashPassword(*user.Password)
		user.Password = &password


		count, err = userCollection.CountDocuments(ctx, bson.M{"phone":user.Phone})
		defer cancel()
		if err!=nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError,gin.H{"error":"error while checking phone"})
		}

		if count>0 {
			c.JSON(http.StatusInternalServerError,gin.H{"error":"email or phone num already exists"})
			return
		}

		user.Created_at, _ = time.Parse(time.RFC3339,time.Now().Format(time.RFC3339))
		user.Updated_at, _ = time.Parse(time.RFC3339,time.Now().Format(time.RFC3339))

		user.ID = primitive.NewObjectID()
		user.User_ID = user.ID.Hex()

		token, refreshToken, _ := helpers.GenerateAllTokens(*user.Email,*user.First_name,*user.Last_name, *user.User_type,*&user.User_ID)

		user.Token = &token
		user.Refresh_token = &refreshToken

		resultInsertionNumber,insertErr := userCollection.InsertOne(ctx,user)
		if insertErr!=nil {
			msg := fmt.Sprintf("User item was not created")
			c.JSON(http.StatusInternalServerError,gin.H{"error":msg})
		}
		defer cancel()
		c.JSON(http.StatusOK,resultInsertionNumber)
	}
}

func Login() gin.HandlerFunc{
	return func(c *gin.Context) {
		ctx,cancel := context.WithTimeout(context.Background(),100*time.Second)

		var user models.User
		var foundUser models.User

		if err := c.BindJSON(&user); err!=nil{
			c.JSON(http.StatusBadRequest,gin.H{"error":err.Error()})
			return
		}

		err := userCollection.FindOne(ctx,bson.M{"email":user.Email}).Decode(&foundUser)
		defer cancel()
		if err!=nil {
			c.JSON(http.StatusInternalServerError,gin.H{"error":"email or pass is incorrect"})
			return
		}

		passwordIsValid, msg := VerifyPassword(*user.Password, *foundUser.Password)
		defer cancel()

		if passwordIsValid != true {
			c.JSON(http.StatusInternalServerError,gin.H{"error":msg})
			return
		}

		if foundUser.Email == nil{
			c.JSON(http.StatusInternalServerError,gin.H{"error":"user not found"})
		}

		token,refreshToken, _ := helpers.GenerateAllTokens(*foundUser.Email, *foundUser.First_name,*foundUser.Last_name,*foundUser.User_type,*&foundUser.User_ID)

		helpers.UpdateAllTokens(token,refreshToken,foundUser.User_ID)

		userCollection.FindOne(ctx,bson.M{"user_id":foundUser.User_ID}).Decode(&foundUser)

		if err!=nil {
			c.JSON(http.StatusInternalServerError,gin.H{"error":err.Error()})
			return
		}
		
		//returning finally
		c.JSON(http.StatusOK,foundUser)
	}
}

func GetUsers() gin.HandlerFunc{
	return func(c *gin.Context) {
		if err := helpers.CheckUserType(c,"ADMIN"); err!=nil{
			c.JSON(http.StatusBadRequest,gin.H{"error":err.Error()})
			return
		}

		ctx,cancel := context.WithTimeout(context.Background(),100*time.Second)

		recordPerPage,err := strconv.Atoi(c.Query("recordPerPage"))

		if err!=nil || recordPerPage<1 {
			recordPerPage=10
		}
		
		page,err1 := strconv.Atoi(c.Query("page"))
		if err1!=nil {
			page =1
		}


		startIndex := (page-1) * recordPerPage
		startIndex,err =  strconv.Atoi(c.Query("startIndex"))

		matchStage := bson.D{{"$match",bson.D{{}}}} //simply a where query
		
		groupStage := bson.D{{"$group", bson.D{ //$group is for grouping
			{"_id", bson.D{{"_id", "null"}}}, //here no grouping is to be done so on the basis of _id(unique)
			{"total_count", bson.D{{"$sum", 1}}}, //we need a count so we will sum up different ids 
			{"data", bson.D{{"$push", "$$ROOT"}}}}}} //$push is to append
		
			//the single data unit retrieved is of type $$ROOT
			//now data will become an array of ROOT type elements

		projectStage := bson.D{ //$project is select query
			{"$project",bson.D{
				{"_id",0}, //first thing shown will be an id
				{"total_count",1}, //another will be total count which will get from groupStage
				{"user_items",bson.D{{"$slice",[]interface{}{"$data",startIndex,recordPerPage}}}},
				
				//now user_items will get array values of data but a slice only
				//starting from 0 till 10 units 
			}},

		}


		result,err := userCollection.Aggregate(ctx, mongo.Pipeline{
			matchStage,groupStage,projectStage,
		})
		defer cancel()
		if err!=nil {
			c.JSON(http.StatusInternalServerError,gin.H{"error":"error occurred while listing users"})
		}

		var allUsers []bson.M
		if err = result.All(ctx,&allUsers); err!=nil {
			log.Fatal(err)
		}

		c.JSON(http.StatusOK,allUsers[0])
	}
}

func GetUser() gin.HandlerFunc{
	return func(c *gin.Context) {
		userId := c.Param("user_id")

		//can only search his own id
		if err:=helpers.MatchUserTypeToUid(c,userId); err != nil {
			c.JSON(http.StatusBadRequest,gin.H{"error":err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(),100*time.Second)

		//search by id
		var user models.User
		err := userCollection.FindOne(ctx,bson.M{"user_id":userId}).Decode(&user)

		defer cancel()

		if err!=nil {
			c.JSON(http.StatusInternalServerError,gin.H{"error":err.Error()})
			return
		}
		c.JSON(http.StatusOK,user)
	}
}