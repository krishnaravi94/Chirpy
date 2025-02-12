package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error){
	hashedPassword,hashError:=bcrypt.GenerateFromPassword([]byte(password),1)
	if hashError!=nil{
		return "",hashError
	}
	return string(hashedPassword),nil
}

func CheckPasswordHash(password, hash string) error{
	checkError:=bcrypt.CompareHashAndPassword([]byte(hash),[]byte(password))
	if checkError!=nil{
		return checkError
	}
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
	issuedDate:=jwt.NewNumericDate(time.Now().UTC())
	expiryDate:=jwt.NewNumericDate(time.Now().UTC().Add(expiresIn))
	newToken:=jwt.NewWithClaims(jwt.SigningMethodHS256,jwt.RegisteredClaims{Issuer: "Chirpy",IssuedAt: issuedDate,ExpiresAt: expiryDate,Subject: userID.String()})
	newSignedToken,tokenSignError:=newToken.SignedString([]byte(tokenSecret))
	if tokenSignError!=nil{
		return "",tokenSignError
	}
	return newSignedToken,nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID,error){
	claims:=&jwt.RegisteredClaims{}
	_,err:=jwt.ParseWithClaims(tokenString,claims,func(token *jwt.Token) (interface{},error){
		return []byte(tokenSecret),nil
	})
	if err !=nil{
		return uuid.Nil,err
	}
	tokenUUID:=claims.Subject
	if tokenUUID==""{
		return uuid.Nil,fmt.Errorf("missing subject in claims")
	}
	return uuid.Parse(tokenUUID)
}