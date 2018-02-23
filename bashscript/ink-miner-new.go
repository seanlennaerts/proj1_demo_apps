package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"

	"./blockartlib"
	"./utils"
	// "math/big"
	"net"
	"net/rpc"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	bae "./blockarterrors"
	"./svgutils"
)

var (
	server *rpc.Client
	// ArtNode         *rpc.Client
	settings        *blockartlib.MinerNetSettings
	serverAddr      string
	minerAddr       net.Addr
	privateKey      *ecdsa.PrivateKey
	publicKey       *ecdsa.PublicKey
	connected       bool
	miners          map[string]*utils.Miner
	numPeers        int64
	blockChain      map[string]*Block
	pendingOps      map[string]*utils.OpRecord
	longestChains   []*Block
	theChosenOne    *Block
	validatingOps   map[string]valOps
	mutexBlockChain sync.Mutex
	directBCMutex   sync.Mutex
)

type notification struct {
	Error error
	Block *Block
}

type valOps struct {
	Channel     chan notification
	ValidateNum uint8
	OpRecord    *utils.OpRecord
}

type Block struct {
	TimeStamp     time.Time
	Nonce         int
	PrevBlockHash string
	Hash          string
	MinerPubKey   *ecdsa.PublicKey
	Ops           []*utils.OpRecord
	Children      []*Block
	Index         int
}

// TODO: Don't need these...??
// type GetChildrenArgs struct {
// 	BlockHash string
// }

// type GetChildrenReply struct {
// 	Children []string
// }

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: [server ip:port] [publicKey] [privateKey]")
		os.Exit(1)
	}
	miners = make(map[string]*utils.Miner)
	blockChain = make(map[string]*Block)
	pendingOps = make(map[string]*utils.OpRecord)
	validatingOps = make(map[string]valOps)
	parseArgs()
	gobRegistration()
	initializeRPC()
	connectToServer()
	time.Sleep(100 * time.Millisecond)
	initializeBlockChain()

	go peerHeartbeat()
	go checkHeartbeats()
	go generateBlocks()

	<-make(chan int)
}

/////////////////////////////////////////////////////////
// General RPC
type RMiner struct{}

func initializeRPC() {
	rMiner := new(RMiner)
	listener, err := utils.RPCServer(utils.GetLocalIP(), rMiner, "Miner")
	utils.CheckError(err)
	minerAddr = listener.Addr()
}

/////////////////////////////////////////////////////////
// Server RPC Communication
func connectToServer() error {
	var err error
	server, err = rpc.Dial("tcp", serverAddr)
	utils.CheckError(err)
	err = server.Call("RServer.Register", utils.MinerInfo{Address: minerAddr, Key: *publicKey}, &settings)
	utils.CheckError(err)
	if err == nil {
		connected = true
		go startHeartbeat()
		go getNodes()
		return nil
	}
	return err
}

// Retrieves list of nodes
func getNodes() {
	var minerIps []net.Addr
	for {
		server.Call("RServer.GetNodes", *publicKey, &minerIps)
		fmt.Println(minerIps)
		for _, address := range minerIps {
			err := connectPeer(address)
			utils.CheckError(err)
			// If connection established numPeers += 1
			// Maybe some alternate goroutine to check who is connected or not
		}
		// TODO establish current connections
		for numPeers < int64(settings.MinNumMinerConnections) {
			time.Sleep(200 * time.Millisecond)
		}
		for numPeers >= int64(settings.MinNumMinerConnections) {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func startHeartbeat() {
	for {
		var result bool
		err := server.Call("RServer.HeartBeat", *publicKey, &result)
		utils.CheckError(err)
		time.Sleep(time.Duration(settings.HeartBeat/5) * time.Millisecond)
	}
}

/////////////////////////////////////////////////////////
// ArtNode RPC Communication
func ArtNodeClientConnect(addr string) (client *rpc.Client) {
	// fmt.Printf("Connecting from client to address %s\n", addr)
	client, err := rpc.Dial("tcp", addr)
	utils.CheckError(err)
	var reply string
	err = client.Call("ArtNode.Idle", "Registration check -- Success", &reply)
	utils.CheckError(err)
	return client
}

/////////////////////////////////////////////////////////
// Miner RPC Communication
func connectPeer(address net.Addr) error {
	addressString := address.String()
	if _, ok := miners[addressString]; ok {
		return nil
	}
	rpcMiner, err := rpc.Dial("tcp", address.String())
	if err != nil {
		return errors.New("Failed to connect to peer")
	}
	miner := &utils.Miner{RpcMiner: rpcMiner, Timestamp: time.Now().UnixNano(), Connected: true}
	miners[addressString] = miner
	var peerKey ecdsa.PublicKey
	err = rpcMiner.Call("Miner.RegisterMiner", &minerAddr, &peerKey)
	if err != nil {
		utils.CheckError(err)
		delete(miners, addressString)
	}
	miner.PublicKey = peerKey
	fmt.Println("Miner connected")
	numPeers++
	return nil
}

func peerHeartbeat() {
	for {
		for _, v := range miners {
			var result bool
			v.RpcMiner.Call("Miner.HeartBeat", utils.MinerInfo{Address: minerAddr, Key: *publicKey}, &result)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func sendBlock(block *Block) bool {
	var response bool
	for _, miner := range miners {
		err := miner.RpcMiner.Call("Miner.SendBlock", block, &response)
		if err != nil {
			utils.CheckError(err)
			continue
		}
	}
	return true
}

/////////////////////////////////////////////////////////
// Miner RPC Methods
func (mrpc *RMiner) Echo(request string, reply *string) (err error) {
	fmt.Println(request)
	return err
}

// This is a "Clone" of connectPeer. Logic could be moved elsewhere
func (mrpc *RMiner) RegisterMiner(address net.Addr, replyKey *ecdsa.PublicKey) error {
	*replyKey = *publicKey
	go connectPeer(address)
	return nil
}

func (mrpc *RMiner) HeartBeat(peerInfo utils.MinerInfo, _ignored *bool) error {
	addressString := peerInfo.Address.String()
	if _, ok := miners[addressString]; ok {
		// Maybe do a comparison on publicKey here.
		miners[addressString].Timestamp = time.Now().UnixNano()
	} else {
		return errors.New("Miner: unknown key")
	}
	return nil
}

// TODO: Get rid of this? - Janik
// func (mrpc *RMiner) GetChildren(request GetChildrenArgs, response *GetChildrenReply) error {
// 	if len(request.BlockHash) > 0 {
// 		block := blockChain[request.BlockHash]
// 		for n, child := range block.Children {
// 			response.Children[n] = child.Hash
// 		}
// 	}
// 	return nil
// }

// Todo this is the call to send a mined block.
func (mrpc *RMiner) SendBlock(block *Block, _response *bool) error {
	fmt.Println("received block")
	err := addBlock(block)
	if utils.CheckError(err) != nil {
		return nil
	}
	// Passes the block along if this miner has not seen the block before.
	for _, miner := range miners {
		_ = miner.RpcMiner.Call("Miner.SendBlock", block, &_response)
	}
	return nil
}

// Dissemination of Operations
func (mrpc *RMiner) SendOperation(opRecord *utils.OpRecord, _response *bool) error {
	// No operation validation required at this point.
	// That should be done at point of mining - Validity may change.
	// Just dont do anything if we already have the op waiting to be added to the longest chain.
	_, ok1 := pendingOps[opRecord.UID]
	_, ok2 := validatingOps[opRecord.UID]
	if ok1 || ok2 {
		return nil
	}
	pendingOps[opRecord.UID] = opRecord
	for _, miner := range miners {
		_ = miner.RpcMiner.Call("Miner.SendOperation", opRecord, &_response)
	}
	return nil
}

//// ArtNode Endpoints ///
func (mrpc *RMiner) RegisterArtNode(request utils.ArtNodeRegistrationRequestPayload, reply *utils.ArtNodeRegistrationReplyPayload) (err error) {
	utils.CheckError(err)
	fmt.Println("Attempting to register node...")
	// fmt.Printf("k: %x\nr: %d\ns:%d\n", publicKey, request.R, request.S)
	valid := ecdsa.Verify(publicKey, request.Hash, request.R, request.S)
	if valid {
		fmt.Println("ACK TRUE")
		// ArtNode = ArtNodeClientConnect(request.ArtNodeAddress)
		reply.ACK = true
		reply.CanvasXMax = settings.CanvasSettings.CanvasXMax
		reply.CanvasYMax = settings.CanvasSettings.CanvasYMax
		fmt.Println(reply.CanvasXMax, reply.CanvasYMax)
	} else {
		fmt.Println("ACK FALSE")
		reply.ACK = false
	}
	return
}

func (mrpc *RMiner) AddShape(request utils.AddShapeRequestPayload, reply *utils.AddShapeResponsePayload) (err error) {
	// TDOD: Fill in real values
	opRecord := utils.OpRecord{
		Op: utils.Op{
			OpType: "add",
			Shape:  request.Shape,
		},
		NodePubKey: publicKey,
	}
	r, s, err := ecdsa.Sign(cryptorand.Reader, privateKey, opToData(opRecord.Op))
	opRecord.OpSig = utils.OpSig{R: r, S: s}
	opRecord.UID = r.String() + s.String()
	fmt.Printf("ADD shape [%s...] with signature [%s...] for key [%s]\n", opRecord.Op.Shape.UID[0:10], opRecord.UID[0:10], fmt.Sprintf("%v", opRecord.NodePubKey)[0:10])
	var _response bool
	(&RMiner{}).SendOperation(&opRecord, &_response)
	notify := make(chan notification, 1)
	validatingOps[opRecord.UID] = valOps{OpRecord: &opRecord, Channel: notify, ValidateNum: request.ValidateNum}
	var notific notification
	notific = <-notify
	if utils.CheckError(notific.Error) != nil {
		var errorCode utils.ErrorCode
		switch notific.Error.(type) {
		case bae.InsufficientInkError:
			errorCode = utils.ErrorCode{Code: utils.InsufficientInk, Num: uint32(notific.Error.(bae.InsufficientInkError))}
		case bae.ShapeOverlapError:
			errorCode = utils.ErrorCode{Code: utils.ShapeOverlap, Msg: string(notific.Error.(bae.ShapeOverlapError))}
		default:
			errorCode = utils.ErrorCode{Code: utils.Custom, Msg: notific.Error.Error()}
		}
		reply.Error = errorCode
		return
	}
	reply.ShapeHash = opRecord.Op.Shape.UID
	reply.BlockHash = notific.Block.Hash
	trace := backTrackChain(theChosenOne, true)
	reply.InkRemaining = getRemainingInkInChain(publicKey, trace)
	return
}

func (mrpc *RMiner) GetGenesisBlockHash(request string, response *string) (err error) {
	*response = settings.GenesisBlockHash
	return
}

func (mrpc *RMiner) GetSvgString(shapeHash string, svgString *string) (err error) {
	longest := backTrackChain(theChosenOne, true)
	opRec := findOpRecWithShapeInChain(shapeHash, longest)
	fmt.Printf("%+v", opRec)
	if opRec != nil && opRec.Op.OpType == "delete" {
		op := findOpRecWithShapeInChain(opRec.Op.OpArg, longest)
		fill := "transparent"
		stroke := "transparent"
		if op.Op.Shape.Svg.Fill != "transparent" {
			fill = "white"
		}
		if op.Op.Shape.Svg.Stroke != "transparent" {
			stroke = "white"
		}
		*svgString = fmt.Sprintf("<path d=\"%s\" stroke=\"%s\" fill=\"%s\"/>", op.Op.Shape.Svg.D, stroke, fill)
	} else if opRec != nil {
		*svgString = fmt.Sprintf("<path d=\"%s\" stroke=\"%s\" fill=\"%s\"/>", opRec.Op.Shape.Svg.D, opRec.Op.Shape.Svg.Stroke, opRec.Op.Shape.Svg.Fill)
	}
	return
}

func (mrpc *RMiner) GetInk(request string, inkRemaining *uint32) (err error) {
	// fmt.Println("Getting ink.************************************")
	longest := backTrackChain(theChosenOne, true)
	*inkRemaining = getRemainingInkInChain(publicKey, longest)
	return
}

func (mrpc *RMiner) DeleteShape(request utils.DeleteShapeRequestPayload, response *utils.DeleteShapeResponsePayload) (err error) {
	// send delete OP
	opRecord := utils.OpRecord{
		Op: utils.Op{
			OpType: "delete",
			OpArg:  request.DeleteHash,
			Shape:  &svgutils.SVGShape{UID: request.UID},
		},
		NodePubKey: publicKey,
	}
	r, s, err := ecdsa.Sign(cryptorand.Reader, privateKey, opToData(opRecord.Op))
	opRecord.OpSig = utils.OpSig{R: r, S: s}
	opRecord.UID = r.String() + s.String()
	fmt.Printf("DELETE shape [%s...] with signature [%s...] for key [%s]\n", opRecord.Op.Shape.UID[0:10], opRecord.UID[0:10], fmt.Sprintf("%v", opRecord.NodePubKey)[0:10])
	var _response bool
	(&RMiner{}).SendOperation(&opRecord, &_response)
	notify := make(chan notification, 1)
	validatingOps[opRecord.UID] = valOps{OpRecord: &opRecord, Channel: notify, ValidateNum: request.ValidateNum}
	var notific notification
	notific = <-notify
	if utils.CheckError(notific.Error) != nil {
		var errorCode utils.ErrorCode
		switch notific.Error.(type) {
		case bae.ShapeOwnerError:
			errorCode = utils.ErrorCode{Code: utils.ShapeOwner, Msg: string(notific.Error.(bae.ShapeOwnerError))}
		default:
			errorCode = utils.ErrorCode{Code: utils.Custom, Msg: notific.Error.Error()}
		}
		response.Error = errorCode
		return
	}
	trace := backTrackChain(theChosenOne, true)
	response.InkRemaining = getRemainingInkInChain(publicKey, trace)
	return
}

func (mrpc *RMiner) GetShapes(blockHash string, shapeHashes *[]string) (err error) {
	longest := backTrackChain(theChosenOne, true)
	block := findBlockInChain(blockHash, longest)
	if block != nil {
		for _, opRec := range block.Ops {
			// if opRec.Op.OpType == "add" {
			*shapeHashes = append(*shapeHashes, opRec.Op.Shape.UID)
			// }
		}
	} else {
		// Awful solution for now
		// TODO: implement error passing
		*shapeHashes = append(*shapeHashes, "INVALID")
	}
	return
}

func (mrpc *RMiner) GetChildren(blockHash string, blockHashes *[]string) (err error) {
	mutexBlockChain.Lock()
	defer mutexBlockChain.Unlock()
	directBCMutex.Lock()
	block, exists := blockChain[blockHash]
	directBCMutex.Unlock()
	if !exists {
		// Awful solution for now
		// TODO: implement error passing
		// note: should return empty []string -- Sean
		*blockHashes = append(*blockHashes, "INVALID")
	} else {
		for _, child := range block.Children {
			*blockHashes = append(*blockHashes, child.Hash)
		}
	}
	return
}

func getBlockchain() map[string]*Block {
	var longestBlockchain = make(map[string]*Block)
	var blockchain map[string]*Block
	var req string // This is an empty var. Not sure what to pass in as request var in this rpc call
	var invalidChain bool
	for _, miner := range miners {
		err := miner.RpcMiner.Call("Miner.GetBlockchain", req, &blockchain)
		utils.CheckError(err)
		if len(blockchain) > len(longestBlockchain) {
			for _, block := range blockChain {
				err := validateBlock(block)
				if err != nil {
					invalidChain = true
					break
				}
			}
			if invalidChain {
				continue
			}
			invalidChain = false
			longestBlockchain = blockchain
		}
	}

	return longestBlockchain
}

func (mrpc *RMiner) GetBlockchain(request string, bc *map[string]*Block) (err error) {
	*bc = blockChain
	return nil
}

/////////////////////////////////////////////////////////
// Blockchain Methods

func initializeBlockChain() {
	// Create genesis block and insert into blockchian
	mutexBlockChain.Lock()
	defer mutexBlockChain.Unlock()
	blockChain = make(map[string]*Block)
	genBlock := &Block{TimeStamp: time.Now(), Nonce: 0, Hash: settings.GenesisBlockHash, Children: []*Block{}, Index: 0}
	directBCMutex.Lock()
	blockChain[genBlock.Hash] = genBlock
	directBCMutex.Unlock()
	longestBlockchain := getBlockchain()
	if len(longestBlockchain) > 0 {
		blockChain = longestBlockchain
	}
	theChosenOne = genBlock
}

func addBlock(block *Block) error {
	mutexBlockChain.Lock()
	err := validateBlock(block)
	if err != nil {
		mutexBlockChain.Unlock()
		return err
	}
	directBCMutex.Lock()
	blockChain[block.Hash] = block
	blockChain[block.PrevBlockHash].Children = append(blockChain[block.PrevBlockHash].Children, block)
	directBCMutex.Unlock()
	updateLongestChain(block)
	mutexBlockChain.Unlock()
	for _, op := range block.Ops {
		delete(pendingOps, op.UID)
	}
	validateAllOps()
	return nil
}

// helper
func validateAllOps() {
	trace := backTrackChain(theChosenOne, true)
	for opSig, ops := range validatingOps {
		_, ok := pendingOps[opSig]
		if !ok {
			// find if on longest chain
			_, index := findOpInChain(opSig, trace)
			if index > -1 {
				fmt.Printf("Validated op in block: %d\n", index)
				if uint8(index) >= ops.ValidateNum {
					delete(validatingOps, opSig)
					ops.Channel <- notification{Block: trace[index], Error: nil}
				}
			} else {
				pendingOps[opSig] = validatingOps[opSig].OpRecord
			}
		}
	}
	fmt.Println("THE BLOCK CHAIN:")
	var l int
	if len(trace) > 5 {
		l = 5
	} else {
		l = len(trace)
	}
	for i := 0; i < l; i++ {
		fmt.Printf("->")
		if len(trace[i].Ops) > 0 {
			fmt.Printf("[*]%v", trace[i].Hash[0:4]) // blocks with ops
		} else {
			fmt.Printf("[ ]%v", trace[i].Hash[0:4])
		}
	}
	fmt.Printf("\n")
}

func (b *Block) computeHash(nonce int, once bool) (int, string) {
	var hash string
	var data []byte
	var difficulty uint8
	h := md5.New()

	if len(b.Ops) == 0 {
		difficulty = settings.PoWDifficultyNoOpBlock
	} else {
		difficulty = settings.PoWDifficultyOpBlock
	}

	for _, record := range b.Ops {
		data = append(data, []byte(fmt.Sprintf("%v", record.Op.Shape))...)
		data = append(data, []byte(record.Op.OpType)...)
		data = append(data, []byte(fmt.Sprintf("%v", record.OpSig))...)
		data = append(data, pubKeyToData(record.NodePubKey)...)
	}
	data = append(data, []byte(b.PrevBlockHash)...)
	data = append(data, pubKeyToData(b.MinerPubKey)...)

	for {
		h.Reset()
		h.Write(data)
		h.Write([]byte(strconv.Itoa(nonce)))
		hash = hex.EncodeToString(h.Sum(nil))
		if utils.TrailingZeroes(hash, int(difficulty)) || once {
			break
		} else {
			nonce++
		}
	}
	return nonce, hash
}

func generateBlocks() {
	for true {
		startBlockCount := len(blockChain)
		block, err := mineBlock()
		utils.CheckError(err)
		if len(block.Ops) == 0 && len(pendingOps) > 0 {
			continue
		}
		// Abort mining if a valid block was added since mining started.
		if startBlockCount != len(blockChain) {
			continue
		}
		// fmt.Printf("keys: %v\n", block.MinerPubKey)
		err = addBlock(block)
		utils.CheckError(err)
		if err == nil {
			fmt.Println("Validation passed and block added.")
			sendBlock(block)
			// fmt.Printf("Mined block UID: %s\nOpcount: %d\n", block.Hash, len(block.Ops))
		}
		printChain(backTrackChain(theChosenOne, true))
		time.Sleep(time.Second * 1)
	}
}

func neighbourIntersectionsHelper(allShapes []*utils.OpRecord) []*utils.OpRecord {
	base := allShapes[0].Op.Shape
	good := []*utils.OpRecord{allShapes[0]}
	for _, opRec := range allShapes[1:] {
		if !reflect.DeepEqual(allShapes[0].NodePubKey, opRec.NodePubKey) {
			for i := 1; i < len(opRec.Op.Shape.AbsPath); i++ {
				var aslice [][]int
				if i == len(opRec.Op.Shape.AbsPath)-1 {
					aslice = opRec.Op.Shape.AbsPath[i-1:]
				} else {
					aslice = opRec.Op.Shape.AbsPath[i-1 : i]
				}
				// fmt.Println(aslice)
				collision := base.CheckIntersection(aslice)
				if collision {
					goto Abort
				}
			}
		}
		good = append(good, opRec)
	Abort:
	}
	fmt.Println(good)
	return good
}

func mineBlock() (*Block, error) {
	blockOps := make([]*utils.OpRecord, 0)
	var allAddShapes []*utils.OpRecord

	for _, op := range pendingOps {
		if op.Op.OpType == "add" {
			allAddShapes = append(allAddShapes, op)
		} else {
			blockOps = append(blockOps, op)
		}
	}

	// now check intersections
	if len(allAddShapes) > 1 {
		blockOps = append(blockOps, neighbourIntersectionsHelper(allAddShapes)...)
	} else { // only one shape so add to blockOps
		blockOps = append(blockOps, allAddShapes...)
	}

	// TODO IF THINGS BREAK UNCOMMENT
	// because it may be redundant work, we think checking intersections is enough

	// valid, err := validateOp(op, theChosenOne)
	// if valid {
	// 	blockOps = append(blockOps, op)
	// 	fmt.Printf("Adding operation [%s...] for shape [%s...] into new block\n", op.UID[0:10], op.Op.Shape.UID[0:10])
	// } else {
	// 	utils.CheckError(err)
	// 	delete(pendingOps, op.UID)
	// }

	block := &Block{TimeStamp: time.Now(), PrevBlockHash: theChosenOne.Hash, MinerPubKey: publicKey, Ops: blockOps}
	nonce := 0
	nonce, hash := block.computeHash(nonce, false)
	block.Hash = hash
	block.Nonce = nonce
	block.Index = theChosenOne.Index + 1
	return block, nil
}

func updateLongestChain(block *Block) {
	switch {
	case block.Index < theChosenOne.Index:
		return
	case block.Index > theChosenOne.Index:
		longestChains = []*Block{block}
		theChosenOne = block
		// fmt.Println(theChosenOne.Index)
	case block.Index == theChosenOne.Index:
		longestChains = append(longestChains, block)
		rand.Seed(time.Now().Unix())
		theChosenOne = longestChains[rand.Int()%len(longestChains)]
	}
	return
}

/////////////////////////////////////////////////////////
// Other Utils
func parseArgs() {
	serverAddr = os.Args[1]

	publicKeyBytes, err := hex.DecodeString(os.Args[2])

	utils.CheckError(err)
	pub, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	utils.CheckError(err)
	switch pub := pub.(type) {
	case *ecdsa.PublicKey:
		publicKey = pub
	default:
		panic("Invalid public key type, must be of type ecdsa.")
	}

	privateKeyBytes, _ := hex.DecodeString(os.Args[3])
	utils.CheckError(err)
	privateKey, err = x509.ParseECPrivateKey(privateKeyBytes)
	utils.CheckError(err)
}

func pubKeyToData(key *ecdsa.PublicKey) []byte {
	var data []byte
	data = append(data, []byte(fmt.Sprintf("%v", key.X))...)
	data = append(data, []byte(fmt.Sprintf("%v", key.Y))...)
	data = append(data, []byte(fmt.Sprintf("%v", key.Curve))...)
	return data
}

func opToData(op utils.Op) []byte {
	var data []byte
	data = append(data, []byte(fmt.Sprintf("%v", op.OpType))...)
	data = append(data, []byte(fmt.Sprintf("%v", op.OpArg))...)
	data = append(data, []byte(fmt.Sprintf("%v", op.Shape))...)
	return data
}

func gobRegistration() {
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})
}

func checkHeartbeats() {
	for {
		for _, v := range miners {
			if time.Now().UnixNano()-v.Timestamp > 2000000000 && v.Connected {
				v.Connected = false
				numPeers--
			} else if time.Now().UnixNano()-v.Timestamp <= 2000000000 && !v.Connected {
				v.Connected = true
				numPeers++
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

///////////////// Blockvalidation //////////////////////////
func validateBlock(block *Block) (err error) {
	// Vaildate genesis block
	if block.Hash == settings.GenesisBlockHash {
		validateGenesisBlock(block)
		return nil
	}
	// Block Validations
	directBCMutex.Lock()
	_, valid := blockChain[block.PrevBlockHash]
	directBCMutex.Unlock()
	if !valid {
		return errors.New("Invalid previous block hash.")
	}

	// TODO: validate index is +1 from previous block
	_, hash := block.computeHash(block.Nonce, true)
	directBCMutex.Lock()
	if _, ok := blockChain[block.PrevBlockHash]; !ok {
		directBCMutex.Unlock()
		return errors.New("Invalid previous block hash.")
	}
	if _, ok := blockChain[block.Hash]; ok {
		directBCMutex.Unlock()
		return errors.New("Block already found on chain.")
	}
	if !utils.TrailingZeroes(hash, int(settings.PoWDifficultyOpBlock)) {
		directBCMutex.Unlock()
		return errors.New("Invalid nonce.")
	}
	directBCMutex.Unlock()

	for _, opRec := range block.Ops {
		valid, err := validateOp(opRec, blockChain[block.PrevBlockHash])

		if !valid {
			return err
		}
	}
	return nil
}

func validateGenesisBlock(gen *Block) (err error) {
	if gen.Ops != nil {
		return errors.New("Invalid genesis block: Contains operations")
	}
	if gen.Nonce != 0 {
		return errors.New("Invalid genesis block: Invalid nonce")
	}
	return nil
}

func validateOp(opRec *utils.OpRecord, parentChain *Block) (bool, error) {
	// fmt.Printf("Validating block-ops, op: %s\n", opRec.UID)
	valid := ecdsa.Verify(opRec.NodePubKey, opToData(opRec.Op), opRec.OpSig.R, opRec.OpSig.S)
	if !valid {
		return false, handleValidationError(opRec, errors.New("corrupted signature detected"))
	}

	trace := backTrackChain(parentChain, true) // starts with previousBlock
	// fmt.Printf("Trace is : %d blocks long.\n\n",len(trace))
	longest := backTrackChain(theChosenOne, true)

	switch {
	// Operation Validations
	case opRec.Op.OpType == "add":
		// Replay Attacks - search only longest chain
		if rec, _ := findOpInChain(opRec.UID, longest); rec != nil {
			return false, handleValidationError(opRec, errors.New("replay attack detected"))
		}
		// Insufficient Ink
		if ink := getRemainingInkInChain(opRec.NodePubKey, trace); ink < uint32(opRec.Op.Shape.Ink) {
			return false, handleValidationError(opRec, bae.InsufficientInkError(ink))
		}
		// Violating intersection policy
		if collision, shapeUID := findIntersectionInChain(opRec, trace); collision {
			return false, handleValidationError(opRec, bae.ShapeOverlapError(shapeUID))
		}
	case opRec.Op.OpType == "delete":
		// Finding non-deleted shape in chain leading to new block
		shape, owner := findShapeInChain(opRec.Op.OpArg, trace)
		if shape == nil {
			return false, handleValidationError(opRec, bae.InvalidShapeHashError(opRec.Op.OpArg))
		}
		if !reflect.DeepEqual(owner, opRec.NodePubKey) {
			return false, handleValidationError(opRec, bae.ShapeOwnerError(opRec.Op.OpArg))
		}
	}
	return true, nil
}

func handleValidationError(opRec *utils.OpRecord, err error) error {
	validationSettings, ok := validatingOps[opRec.UID]
	if ok {
		validationSettings.Channel <- notification{Error: err}
	}
	delete(validatingOps, opRec.UID)
	delete(pendingOps, opRec.UID)
	return err
}

func getRemainingInkInChain(pubKey *ecdsa.PublicKey, trace []*Block) (remainingInk uint32) {
	deletedShapes := map[string]bool{}
	for _, block := range trace {
		// fmt.Printf("keys: block: %v\n", block.MinerPubKey, pubKey)
		if reflect.DeepEqual(block.MinerPubKey, pubKey) {
			if len(block.Ops) == 0 {
				remainingInk += settings.InkPerNoOpBlock
			} else {
				remainingInk += settings.InkPerOpBlock
			}

			for _, opRec := range block.Ops {
				if opRec.Op.OpType == "delete" {
					deletedShapes[opRec.Op.OpArg] = true
				}
				if opRec.Op.OpType == "add" && !deletedShapes[opRec.Op.Shape.UID] {
					remainingInk -= uint32(opRec.Op.Shape.Ink)
				}
			}
		}
	}
	return remainingInk
}

func findBlockInChain(blockHash string, chain []*Block) *Block {
	for _, block := range chain {
		if block.Hash == blockHash {
			return block
		}
	}
	return nil
}

// PLS TEST ME (the logic is 'smart' -- as in dangerous)
func findShapeInChain(shapeHash string, chain []*Block) (*svgutils.SVGShape, *ecdsa.PublicKey) {
	// deleted := false
	for _, block := range chain {
		for _, opRec := range block.Ops {
			if opRec.Op.OpArg == shapeHash && opRec.Op.OpType == "delete" {
				// deleted = true
				fmt.Println("Shouldn't see this 1l2i3j2lj")
				return nil, nil
			}
			fmt.Printf("%s == %s", opRec.Op.Shape.UID, shapeHash)
			if opRec.Op.Shape.UID == shapeHash && opRec.Op.OpType == "add" {
				// if the shape hasn't been removed, then return it
				return opRec.Op.Shape, opRec.NodePubKey
			}
		}
	}
	return nil, nil
}

func findOpRecWithShapeInChain(shapeHash string, chain []*Block) *utils.OpRecord {
	deleted := false
	for _, block := range chain {
		for _, opRec := range block.Ops {
			if opRec.Op.Shape.UID == shapeHash && !deleted {
				// if the shape hasn't been removed, then return it
				return opRec
			}
		}
	}
	return nil
}

// Returns opRecord and the index of the block in the chain that the record was found in
func findOpInChain(opUID string, chain []*Block) (*utils.OpRecord, int) {
	for index, block := range chain {
		for _, opRec := range block.Ops {
			if opRec.UID == opUID {
				return opRec, index
			}
		}
	}
	return nil, -1
}

func findIntersectionInChain(record *utils.OpRecord, chain []*Block) (bool, string) {
	deletedHashes := make(map[string]bool)
	shape := record.Op.Shape
	for _, block := range chain {
		for _, opRecord := range block.Ops {
			if opRecord.Op.OpType == "delete" {
				deletedHashes[opRecord.Op.OpArg] = true
				continue
			}
			_, ok := deletedHashes[opRecord.UID]
			if opRecord.Op.OpType == "add" && !ok && !reflect.DeepEqual(opRecord.NodePubKey, record.NodePubKey) {
				for i := 1; i < len(opRecord.Op.Shape.AbsPath); i++ {
					var aslice [][]int
					if i == len(opRecord.Op.Shape.AbsPath)-1 {
						aslice = opRecord.Op.Shape.AbsPath[i-1:]
					} else {
						aslice = opRecord.Op.Shape.AbsPath[i-1 : i+1]
					}
					fmt.Println(aslice)
					collision := shape.CheckIntersection(aslice)
					if collision {
						return true, opRecord.Op.Shape.UID
					}
				}
			}
		}
	}
	return false, ""
}

func backTrackChain(block *Block, includeFirst bool) (trace []*Block) {
	if !includeFirst {
		directBCMutex.Lock()
		block = blockChain[block.PrevBlockHash]
		directBCMutex.Unlock()
	}
	// fmt.Printf("Genesis: %s\n", settings.GenesisBlockHash)
	for block.PrevBlockHash != "" {
		directBCMutex.Lock()
		trace = append(trace, blockChain[block.Hash])
		block = blockChain[block.PrevBlockHash]
		directBCMutex.Unlock()
	}
	directBCMutex.Lock()
	trace = append(trace, blockChain[block.Hash])
	directBCMutex.Unlock()
	return
}

func printChain(trace []*Block) {
	for _, block := range trace {
		if len(block.Ops) > 0 {
			fmt.Printf("====== Block %s, Depth: %d ========\n  Ops: [\n", block.Hash, block.Index)
			for _, opRec := range block.Ops {
				fmt.Printf("%s...", opRec.UID[0:10])
			}
			fmt.Printf("]\n")
		}
	}
}
