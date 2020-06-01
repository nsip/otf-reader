package util

import (
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"regexp"

	"github.com/nats-io/nuid"
	hashids "github.com/speps/go-hashids"
)

//
// checks provided nats topic only has alphanumeric & dot separators within the name
//
var topicRegex = regexp.MustCompile("^[A-Za-z0-9]([A-Za-z0-9.]*[A-Za-z0-9])?$")

//
// generate a unique id - nuid in this case
//
func GenerateID() string {

	return nuid.Next()

}

//
// generate a short useful unique name - hashid in this case
//
func GenerateName() string {

	name := "reader"

	// generate a random number
	number0, err := rand.Int(rand.Reader, big.NewInt(10000000))

	hd := hashids.NewData()
	hd.Salt = "otf-reader random name generator 2020"
	hd.MinLength = 5
	h, err := hashids.NewWithData(hd)
	if err != nil {
		log.Println("error auto-generating name: ", err)
		return name
	}
	e, err := h.EncodeInt64([]int64{number0.Int64()})
	if err != nil {
		log.Println("error encoding auto-generated name: ", err)
		return name
	}
	name = e

	return name

}

//
// do regex check on topic names provided for nats
//
func ValidateNatsTopic(tName string) (bool, error) {

	valid := topicRegex.Match([]byte(tName))
	if valid {
		return valid, nil
	}
	return false, errors.New("Nats topic names must be alphanumeric only, can also contain (but not start or end with) period ( . ) as token delimiter.")

}
