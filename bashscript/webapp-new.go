/*
	This art app will output the html file. It will also serve a website that allows
	the user to traverse the canvas' history block-by-block

	Time permitting: it will also allow the user to input a path (via text input) and add
	new shape to the canvas
*/

package main

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"./blockartlib"
	"./utils"
)

type page struct {
	CanvasWidth  uint32
	CanvasHeight uint32
	Shapes       template.HTML
}

var (
	p           page
	Canvas      blockartlib.Canvas
	svgStrings  []string
	validateNum uint8 = 2
)

func getSvgStrings() string {
	svgStrings = make([]string, 0)
	genHash, err := Canvas.GetGenesisBlock()
	fmt.Println(genHash)

	hashesOnLongestChain, err := getLongest(genHash)
	utils.CheckError(err)

	for _, h := range hashesOnLongestChain {
		shapeHashes, err := Canvas.GetShapes(h)
		utils.CheckError(err)
		for _, s := range shapeHashes {
			svgStr, err := Canvas.GetSvgString(s)
			utils.CheckError(err)
			svgStrings = append(svgStrings, svgStr)
		}
	}
	for i := len(svgStrings)/2 - 1; i >= 0; i-- {
		opp := len(svgStrings) - 1 - i
		svgStrings[i], svgStrings[opp] = svgStrings[opp], svgStrings[i]
	}

	// reversedString := make([]string, 0)
	// for i := len(reversedString) - 1; i >= 0; i-- {
	// 	reversedString = append(reversedString, svgStrings[i])
	// }

	allStrings := strings.Join(svgStrings, "\n")
	return allStrings
}

func handler(w http.ResponseWriter, r *http.Request) {
	p.Shapes = template.HTML(getSvgStrings())

	t, _ := template.ParseFiles("./webapp/index.html")
	t.Execute(w, p)
}

func addShapeHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Println(r.FormValue("path"))
	fmt.Println(r.FormValue("fill"))
	fmt.Println(r.FormValue("stroke"))

	_, _, _, err := Canvas.AddShape(validateNum, blockartlib.PATH, r.FormValue("path"), r.FormValue("fill"), r.FormValue("stroke"))
	utils.CheckError(err)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("%s", err)))
	} else {
		// svgToInject := getSvgStrings()
		w.Write([]byte(""))
	}

}

func getLongest(hash string) (longest []string, err error) {
	children, err := Canvas.GetChildren(hash)
	utils.CheckError(err)

	var l int
	for _, c := range children {
		children2, err2 := getLongest(c)
		if len(children2) > l {
			longest = children2
			l = len(children2)
		}
		err = err2
	}
	longest = append(longest, hash)
	return
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: [miner ip:port] [privateKey] [port]")
		os.Exit(1)
	}
	minerAddr := os.Args[1]
	bytes, err := hex.DecodeString(os.Args[2])
	utils.CheckError(err)
	privateKey, err := x509.ParseECPrivateKey(bytes)
	utils.CheckError(err)

	// open canvas
	canvas, settings, err := blockartlib.OpenCanvas(minerAddr, *privateKey)
	Canvas = canvas
	fmt.Println(settings)
	p = page{CanvasWidth: settings.CanvasXMax, CanvasHeight: settings.CanvasYMax}

	http.HandleFunc("/", handler)
	http.HandleFunc("/addshape", addShapeHandler)
	fmt.Println("http://localhost:" + os.Args[3])
	err = http.ListenAndServe(":"+os.Args[3], nil)
	utils.CheckError(err)
}
