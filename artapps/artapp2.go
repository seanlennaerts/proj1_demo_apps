package main

import (
  "fmt"
  "bufio"
  "os"
  bal "../../proj1_k4c9_u1d0b_v3c0b_y3b9/blockartlib"
  "crypto/x509"
  "encoding/hex"
  // "strconv"
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
  if err != nil {
      fmt.Print(err)
      os.Exit(1)
  }

  fmt.Printf(
`Canvas Open.
`)

  // VALID ADD //
  fmt.Printf(
`Adding simple, non-transparent stroke
[M 50 100 l 20 20] ---
`)
  stallForUser("")  
  go waitingDots(waitingChannel)
  validateNum = 4
  shapeHash, _, _, err := canvas.AddShape(validateNum, bal.PATH, "M 50 100 l 20 20", "transparent", "red")
  waitingChannel <- true

  if err != nil && err.Error() != "replay attack detected"{
    fmt.Print(err)
    os.Exit(1)
  }

  fmt.Println("Created shape with hash " + shapeHash)


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