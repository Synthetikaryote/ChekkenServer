package main

import (
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"time"
	"bytes"
	"encoding/binary"
	"io"
)

// --- gameplay-specific code ---

type Vec [3]float64

func (res *Vec) Add(a, b *Vec) *Vec {
	(*res)[0] = (*a)[0] + (*b)[0]
	(*res)[1] = (*a)[1] + (*b)[1]
	(*res)[2] = (*a)[2] + (*b)[2]
	return res
}

func (res *Vec) Sub(a, b *Vec) *Vec {
	(*res)[0] = (*a)[0] - (*b)[0]
	(*res)[1] = (*a)[1] - (*b)[1]
	(*res)[2] = (*a)[2] - (*b)[2]
	return res
}

func (a *Vec) Equals(b *Vec) bool {
	for i := range *a {
		if (*a)[i] != (*b)[i] {
			return false
		}
	}
	return true
}

type Model uint32

const (
	Paddle Model = 1
	Ball Model = 2
)

type Entity struct {
	pos, vel, size Vec
	model Model
}

var ents = make([]Entity, 3)
// keep the state from last state to send the diff to the client
var entsOld = make([]Entity, 3)

// called automatically when the server starts
func init() {
	ents[0].model = Paddle
	ents[0].pos = Vec{-75, 0, 0}
	ents[0].size = Vec{5, 20, 10}
	ents[1].model = Paddle
	ents[1].pos = Vec{75, 0, 0}
	ents[1].size = Vec{5, 20, 10}
	ents[2].model = Ball
	ents[2].size = Vec{20, 20, 20}
}

const (
	Up Action = 0
	Down Action = 1
)

func copyState() {
	for i, ent := range ents {
		entsOld[i] = ent
	}
}

func updateSimulation() {

}

func serialize(buf io.Writer, serAll bool) {
	bitMask := make([]byte, 1)
	bufTemp := &bytes.Buffer{}
	for i, ent := range ents {
		if serAll || ent.model != entsOld[i].model {
			bitMask[0] |= 1 << uint(i)
			binary.Write(bufTemp, binary.LittleEndian, ent.model)
		}
	}
	buf.Write(bitMask)
	buf.Write(bufTemp.Bytes())

	bitMask[0] = 0
	bufTemp.Reset()
	for i, ent := range ents {
		if serAll || ent.pos != entsOld[i].pos {
			bitMask[0] |= 1 << uint(i)
			binary.Write(bufTemp, binary.LittleEndian, ent.pos)
		}
	}
	buf.Write(bitMask)
	buf.Write(bufTemp.Bytes())

	bitMask[0] = 0
	bufTemp.Reset()
	for i, ent := range ents {
		if serAll || ent.vel != entsOld[i].vel {
			bitMask[0] |= 1 << uint(i)
			binary.Write(bufTemp, binary.LittleEndian, ent.vel)
		}
	}
	buf.Write(bitMask)
	buf.Write(bufTemp.Bytes())

	bitMask[0] = 0
	bufTemp.Reset()
	for i, ent := range ents {
		if serAll || ent.size != entsOld[i].size {
			bitMask[0] |= 1 << uint(i)
			binary.Write(bufTemp, binary.LittleEndian, ent.size)
		}
	}
	buf.Write(bitMask)
	buf.Write(bufTemp.Bytes())
}

// --- end of gameplay-specific code ---

type PlayerId uint32

type UserCommand struct {
	Actions uint32
}

type Action uint32

type ClientConn struct {
	ws *websocket.Conn
	inBuf [1500]byte
	currentCmd UserCommand
	cmdBuf chan UserCommand
}

var newConn = make(chan *ClientConn)
var clients = make(map[PlayerId]*ClientConn)

var maxId = PlayerId(0)
func newId() PlayerId {
	maxId++
	return maxId
}

func active(id PlayerId, action Action) bool {
	if (clients[id].currentCmd.Actions & (1 << action)) > 0 {
		return true
	}
	return false
}

func wsHandler(ws *websocket.Conn) {
	log.Println("incoming connection")

	cl := &ClientConn{}
	cl.ws = ws
	cl.cmdBuf = make(chan UserCommand, 5)

	cmd := UserCommand{}


	newConn <- cl
	for {
		pkt := cl.inBuf[0:]
		n, err := ws.Read(pkt)
		pkt = pkt[0:n]
		if err != nil {
			log.Println(err)
			break
		}
		buf := bytes.NewBuffer(pkt)
		err = binary.Read(buf, binary.LittleEndian, &cmd)
		if err != nil {
			log.Println(err)
			break
		}
		cl.cmdBuf <- cmd
	}
}

func updateInputs() {
	for _, cl := range clients {
		for {
			select {
				case cmd := <-cl.cmdBuf:
					cl.currentCmd = cmd
				default:
					goto done
			}
		}
	done:
	}
}

var removeList = make([]PlayerId, 3)
func sendUpdates() {
	buf := &bytes.Buffer{}
	serialize(buf, false)
	removeList = removeList[0:0]
	for id, cl := range clients {
		err := websocket.Message.Send(cl.ws, buf.Bytes())
		if err != nil {
			removeList = append(removeList, id)
			log.Println(err)
		}
	}
	copyState()
	for _, id := range removeList {
		//disconnect(id)
		delete(clients, id)
	}
}

func main() {
	http.Handle("/", websocket.Handler(wsHandler))
	http.Handle("/www/", http.StripPrefix("/www/",
		http.FileServer(http.Dir("./www"))))
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	http.Handle("/ShootDeploy/", http.StripPrefix("/ShootDeploy/",
		http.FileServer(http.Dir("./ShootDeploy"))))
	go func() {
		log.Fatal(http.ListenAndServe(":8090", nil))
	}()

	//running at 30 FPS
	frameNS := time.Duration(int(1e9) / 30)
	clk := time.NewTicker(frameNS)

	//main loop
	for {
		select {
			case <-clk.C:
				updateInputs()
				updateSimulation()
				sendUpdates()
			case cl := <-newConn:
				log.Println("Incoming connection")
				id := newId()
				clients[id] = cl
				//login(id)
				buf := &bytes.Buffer{}
				serialize(buf, true)
				websocket.Message.Send(cl.ws, buf.Bytes())
		}
	}
}