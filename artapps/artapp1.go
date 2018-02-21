package main

import (
  "fmt"
  "bufio"
  "os"
  bal "../../proj1_k4c9_u1d0b_v3c0b_y3b9/blockartlib"
  "crypto/x509"
  "encoding/hex"
  "strconv"
  "time"

)

var (
  minerAddress        string
  privateKeyString    string
  err                 error
  reader              bufio.Reader
  validateNum         uint8
  waitingChannel      chan bool
)


func main() {

  if len(os.Args) < 3 {
    fmt.Println("Usage: [miner ip:port] [privateKey]")
    os.Exit(1)
  } else {
    minerAddress =  os.Args[1]
    privateKeyString = os.Args[2]
  }

  keyBytes, err := hex.DecodeString(privateKeyString )
  if err != nil {
    fmt.Println("Invalid Key --- please check.")
    os.Exit(1)
  }
  privateKey, err := x509.ParseECPrivateKey(keyBytes)
  if err !=nil {
    fmt.Println("Invalid Key --- please check.")
    os.Exit(1)  
  }

  reader = *bufio.NewReader(os.Stdin)
  waitingChannel = make(chan bool)
  fmt.Printf(
`Project 1 Demo Application
!!! USE WITH CAUTION !!!

Commencing Test Sequence:
`)

  fmt.Printf(
`Opening Canvas:
`)


  canvas, _, err := bal.OpenCanvas(minerAddress, *privateKey)
  // canvas.CloseCanvas()
  fmt.Printf(
`Canvas Open.
`)

// VALID ADD //
  fmt.Printf(
`Adding simple, non-transparent stroke from ` + buildCoordinates(0,0) + `to ` + buildCoordinates(20,20) +`
[M 0 0 L 20 20] ---
`)
  stallForUser("")  
  go waitingDots(waitingChannel)
  validateNum = 4
  shapeHash, _, _, err := canvas.AddShape(validateNum, bal.PATH, "M 0 0 L 20 20", "transparent", "red")
  waitingChannel <- true

  if err != nil {
    fmt.Print(err)
    os.Exit(1)
  }

// INSUFFICIENT INK //
  fmt.Printf(
`Adding HUGE shape 
[M 0 0 v 1000 h 1000 v -1000 h -1000]
`)
  stallForUser("")
  go waitingDots(waitingChannel)
  _, _, _, err = canvas. AddShape(validateNum, bal.PATH, "M 0 0 v 1000 h 1000 v -1000 h -1000", "blue", "yellow")
  waitingChannel  <- true
  catchError(err)

// VALID DELETE //
  fmt.Printf(
`Deleting the previous shape [#` + shapeHash  + `] ---
`)
  stallForUser("")
  go waitingDots(waitingChannel)
  validateNum = 4
  _, err = canvas.DeleteShape(validateNum, shapeHash)
  waitingChannel <- true
  if err !=nil {
    fmt.Print(err)
    os.Exit(1)
  }

// INVALID SHAPE DELETE //
      fmt.Printf(
`Deleting the shape again [#` + shapeHash  + `]
`)
  stallForUser("")
  go waitingDots(waitingChannel)
  _, err = canvas.DeleteShape(validateNum, shapeHash)
  waitingChannel  <- true
  catchError(err)


// STRING TOO LONG //
  var longString string
  for i:=0; i < 130; i++ {
    longString = longString + "M " + strconv.Itoa(i) + " " + strconv.Itoa(i)
  }
  fmt.Printf(
`Passing a string that is too long --
` + longString + `
`)
  stallForUser("")
  shapeHash, _, _, err = canvas.AddShape(validateNum, bal.PATH, longString, "transparent", "red")
  catchError(err)

// OUT OF BOUNDS //
  fmt.Printf(
`Adding shape off bounds
"M 5 5 l -100 10"
`)
  stallForUser("")
  shapeHash, _, _, err = canvas.AddShape(validateNum, bal.PATH, "M 5 5 l -100 10", "transparent", "red")
  catchError(err)

// DELETING ANOTHER USER'S SHAPE //
  fmt.Printf(
`Deleting shape of other node.
`)

  shapeHash2 := stallForUser("ShapeHash [INPUT]:")

  go waitingDots(waitingChannel)
  _, err = canvas.DeleteShape(validateNum, shapeHash2)
  waitingChannel  <- true

  catchError(err)
}

func buildCoordinates(x, y int) string {
  return fmt.Sprintf("(x,y): [%d,%d]", x, y) 
}

func stallForUser(prompt string) string {
  if prompt == "" {
    fmt.Print("Hit RETURN to continue... ")
  } else {
    fmt.Println(prompt)
  }
  text, _ := reader.ReadString('\n') 
  return text[:len(text) -1]
}

func catchError(err error) {
  if err != nil {
    fmt.Printf(
`!!! ERROR CAUGHT: !!!
` + err.Error() + `

`  )
  }
}

func waitingDots(done chan bool) {
  for true {
    time.Sleep(time.Second * 1)
    select {
      case <- done:
        fmt.Printf("\n\n")
        goto DONE
      default:
        fmt.Printf(".")
    }
  }
DONE:
  return
}