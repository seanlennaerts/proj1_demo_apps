package main

import (
  "fmt"
  "bufio"
  "os"
  bal "../../proj1_k4c9_u1d0b_v3c0b_y3b9/blockartlib"
  "crypto/x509"
  "encoding/hex"
  "strconv"
  // "time"

)

var (
  minerAddress        string
  privateKeyString    string
  err                 error
  reader              bufio.Reader
  validateNum         uint8
  waitingChannel      chan bool
  x                   int
  y                   int
  color               string
)


func main() {

  if len(os.Args) < 6 {
    fmt.Println("Usage: [miner ip:port] [privateKey] [x] [y] [color]")
    os.Exit(1)
  } else {
    minerAddress =  os.Args[1]
    privateKeyString = os.Args[2]
    x, _ = strconv.Atoi(os.Args[3])
    y, _ = strconv.Atoi(os.Args[4])
    color = os.Args[5]
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

  // ADD SQUARE //
  fmt.Printf(
`Adding square...
`)
  // stallForUser("")  
  // go waitingDots(waitingChannel)
  validateNum = 4
  shapeHash, _, _, err := canvas.AddShape(validateNum, bal.PATH, fmt.Sprintf("M %d %d h 10 v 10 h -10 z", x, y), color, "red")
  // waitingChannel <- true

  if err != nil && err.Error() != "replay attack detected"{
    fmt.Print(err)
    os.Exit(1)
  }

  fmt.Println("Created shape with hash " + shapeHash)


  // ADD LINE //
  // ADD SQUARE //
  fmt.Printf(
`Adding line 1...
`)
  // stallForUser("")  
  // go waitingDots(waitingChannel)
  validateNum = 4
  shapeHash, _, _, err = canvas.AddShape(validateNum, bal.PATH, fmt.Sprintf("M %d %d h 50", x, y + 20), "transparent" , color)
  // waitingChannel <- true

  if err != nil && err.Error() != "replay attack detected"{
    fmt.Print(err)
    os.Exit(1)
  }

    fmt.Println("Created shape with hash " + shapeHash)



  // ADD LINE // 
    // ADD SQUARE //
  fmt.Printf(
`Adding line 2...
`)
  // stallForUser("")  
  // go waitingDots(waitingChannel)
  validateNum = 4 
   shapeHash, _, _, err = canvas.AddShape(validateNum, bal.PATH, fmt.Sprintf("M %d %d h 50", x, y + 20), "transparent", color)
  // waitingChannel <- true

  if err != nil && err.Error() != "replay attack detected"{
    fmt.Print(err)
    os.Exit(1)
  }

    fmt.Println("Created shape with hash " + shapeHash)

}


// func stallForUser(prompt string) string {
//   if prompt == "" {
//     fmt.Print("Hit RETURN to continue... ")
//   } else {
//     fmt.Println(prompt)
//   }
//   text, _ := reader.ReadString('\n') 
//   return text[:len(text) -1]
// }

// func catchError(err error) {
//   if err != nil {
//     fmt.Printf(
// `!!! ERROR CAUGHT: !!!
// ` + err.Error() + `

// `  )
//   }
// }

// func waitingDots(done chan bool) {
//   for true {
//     time.Sleep(time.Second * 1)
//     select {
//       case <- done:
//         fmt.Printf("\n\n")
//         goto DONE
//       default:
//         fmt.Printf(".")
//     }
//   }
// DONE:
//   return
// }