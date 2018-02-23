/*

A trivial application to illustrate how the blockartlib library can be
used from an application in project 1 for UBC CS 416 2017W2.

Usage:
go run art-app.go
*/

package main

// Expects blockartlib.go to be in the ./blockartlib/ dir, relative to
// this art-app.go file
import (
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"time"

	"./blockartlib"
	"./utils"
)

// import "crypto/ecdsa"

func main() {

	if len(os.Args) < 3 {
		fmt.Println("Usage: [miner ip:port] [privateKey] [ink colour]")
		os.Exit(1)
	}

	minerAddr := os.Args[1]
	bytes, err := hex.DecodeString(os.Args[2])
	utils.CheckError(err)
	privateKey, err := x509.ParseECPrivateKey(bytes)
	utils.CheckError(err)

	// Open a canvas.
	fmt.Println("Opening canvas")
	canvas, _, err := blockartlib.OpenCanvas(minerAddr, *privateKey)
	if utils.CheckError(err) != nil {
		return
	} else {
		fmt.Println("Opened Canvas")
	}

	validateNum := uint8(2)

	colour := os.Args[3]

	for {
		// Generate random coordinates for lines
		rand.Seed(time.Now().Unix())
		x := rand.Int() % 1200
		y := rand.Int() % 1200
		z := rand.Int() % 1200
		a := rand.Int() % 1200

		shape := fmt.Sprintf("M %d %d L %d %d", x, y, a, z)

		_, _, _, err := canvas.AddShape(validateNum, blockartlib.PATH, shape, "transparent", colour)
		if err != nil {
			utils.CheckError(err)
		} else {
			fmt.Println("Added shape")
		}
		ink, _ := canvas.GetInk()
		fmt.Printf("Ink remaining: %d\n", ink)
		time.Sleep(5 * time.Second)

	}

	canvas.CloseCanvas()

}
