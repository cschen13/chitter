package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type Msg struct {
	id   int
	line []byte
}

type PrivateMsg struct {
	msg        Msg
	recpientId int
}

func main() {
	isClient := flag.Bool("c", false, "Connect as a client")
	flag.Parse()
	if len(flag.Args()) == 1 {
		port := flag.Args()[0]
		if *isClient {
			conn, err := net.Dial("tcp", ":"+port)
			if err != nil {
				println("Failed to connect to server: " + err.Error())
				return
			}
			fmt.Printf("Connection established: %v <-> %v\n", conn.LocalAddr(), conn.RemoteAddr())
			chatClient(conn)
		} else {
			fmt.Println("Starting a new server...")
			chatServer(port)
		}
	} else {
		fmt.Println("usage: go run chitter.go [-c] <port_number>")
	}
}

func chatClient(server net.Conn) {
	stdinChan := make(chan []byte)
	serverChan := make(chan []byte)
	quit := make(chan bool)
	go readSelect(bufio.NewReader(os.Stdin), stdinChan, quit)
	go readSelect(bufio.NewReader(server), serverChan, quit)
	defer server.Close()
	for {
		select {
		case line := <-stdinChan:
			server.Write(line)
		case serverResp := <-serverChan:
			fmt.Print(string(serverResp))
		case <-quit:
			fmt.Println("Connection was closed...")
			return
		}
	}
}

func chatServer(port string) {
	server, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic("Failed to start listening: " + err.Error())
	}

	fmt.Println("Listening on port " + port)
	id := 0
	idChannelMap := make(map[int]chan []byte)
	newConnChan := make(chan net.Conn)
	broadcastChan := make(chan Msg)
	privateChan := make(chan PrivateMsg)
	go acceptConnections(server, newConnChan)
	for {
		select {
		case client := <-newConnChan:
			idChannelMap[id] = make(chan []byte)
			fmt.Printf("New client %d\n", id)
			go handleClient(id, client, idChannelMap[id], broadcastChan, privateChan)
			id++
		case broadcast := <-broadcastChan:
			for channelID, channel := range idChannelMap {
				if channelID != broadcast.id {
					broadcastMsg := []byte(strconv.Itoa(broadcast.id) + ": ")
					channel <- append(broadcastMsg, broadcast.line...)
				}
			}
		case pm := <-privateChan:
			//Removing a client who has quit the connection
			//Client handler should send PM with recipient ID of -1
			if pm.recpientId == -1 {
				close(idChannelMap[pm.msg.id])
				delete(idChannelMap, pm.msg.id)
			} else {
				if channel, ok := idChannelMap[pm.recpientId]; ok {
					channel <- append([]byte(strconv.Itoa(pm.msg.id)+": "), pm.msg.line...)
					fmt.Printf("Sent private message to %d\n", pm.msg.id)
				} else {
					idChannelMap[pm.msg.id] <- []byte("chitter: No active user with ID " + strconv.Itoa(pm.recpientId) + "\n")
				}
			}
		}
	}
}

func acceptConnections(server net.Listener, newConnChan chan net.Conn) {
	for {
		client, err := server.Accept()
		if err != nil {
			fmt.Println("Couldn't accept: " + err.Error())
			continue
		}
		fmt.Printf("Connection established: %v <-> %v\n", client.LocalAddr(), client.RemoteAddr())
		newConnChan <- client
	}
}

func handleClient(id int, client net.Conn, recvChan chan []byte, broadcastChan chan Msg, privateChan chan PrivateMsg) {
	clientChan := make(chan []byte)
	quit := make(chan bool)
	go readSelect(bufio.NewReader(client), clientChan, quit)
	defer client.Close()
	for {
		select {
		case sendMsg := <-clientChan:
			if cmdIndex := strings.Index(string(sendMsg), ":"); cmdIndex == -1 {
				broadcastChan <- Msg{id, []byte(trimSpaceLeft(string(sendMsg)))}
			} else {
				switch cmd := strings.TrimSpace(string(sendMsg)[:cmdIndex]); cmd {
				case "whoami":
					whoamiMsg := []byte("chitter: " + strconv.Itoa(id) + "\n")
					client.Write(whoamiMsg)
				case "all":
					broadcastChan <- Msg{id, []byte(trimSpaceLeft(string(sendMsg)[cmdIndex+1:]))}
				default:
					if pmID, err := strconv.Atoi(cmd); err == nil {
						if pmID == id {
							fmt.Printf("Ignoring self-private message from client %d\n", id)
							client.Write([]byte("chitter: Did you really just try to PM yourself?\n"))
						} else {
							privateChan <- PrivateMsg{Msg{id, []byte(trimSpaceLeft(string(sendMsg)[cmdIndex+1:]))}, pmID}
						}
					} else {
						fmt.Printf("Unrecognized command from client %d\n", id)
					}
				}
			}
		case rcvdMsg := <-recvChan:
			client.Write(rcvdMsg)
		case <-quit:
			fmt.Printf("Client %d closed\n", id)
			privateChan <- PrivateMsg{Msg{id, nil}, -1}
			return
		}
	}
}

func readSelect(reader *bufio.Reader, ch chan []byte, quitChan chan bool) {
	for {
		switch line, err := reader.ReadBytes('\n'); err {
		case nil:
			ch <- line
		case io.EOF:
			quitChan <- true
			return
		default:
			fmt.Println("Error while reading: ", err.Error())
		}
	}
}

func trimSpaceLeft(s string) string {
	return strings.TrimLeft(s, " \t")
}
