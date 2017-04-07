package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type msg struct {
	id   int
	line []byte
}

// TODO: Pass a flag in to denote a client instead of a server
// Currently, we try to connect and start a server if we can't
func main() {
	if len(os.Args) == 2 {
		port := os.Args[1]
		conn, err := net.Dial("tcp", ":"+port)
		if err != nil {
			fmt.Println("Failed to connect to server: " + err.Error())
			fmt.Println("Starting a new one...")
			chatServer(port)
		} else {
			fmt.Printf("Connection established: %v <-> %v\n", conn.LocalAddr(), conn.RemoteAddr())
			chatClient(conn)
		}
	} else {
		fmt.Println("usage: go run chitter.go <port_number>")
	}
}

func chatClient(server net.Conn) {
	stdinChan := make(chan []byte)
	serverChan := make(chan []byte)
	quit := make(chan bool)
	go clientSelect(bufio.NewReader(os.Stdin), stdinChan, quit)
	go clientSelect(bufio.NewReader(server), serverChan, quit)
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

func clientSelect(reader *bufio.Reader, ch chan []byte, quitChan chan bool) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				quitChan <- true
				return
			} else {
				fmt.Println("Error while reading: ", err.Error())
				continue
			}
		}
		ch <- line
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
	broadcastChan := make(chan msg)
	go acceptConnections(server, newConnChan)
	for {
		select {
		case client := <-newConnChan:
			idChannelMap[id] = make(chan []byte)
			fmt.Printf("New client %d\n", id)
			go handleClient(id, client, idChannelMap[id], broadcastChan)
			id++
		case broadcast := <-broadcastChan:
			for channelID, channel := range idChannelMap {
				if channelID != broadcast.id {
					broadcastMsg := []byte(strconv.Itoa(broadcast.id) + ": ")
					channel <- append(broadcastMsg, broadcast.line...)
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

func handleClient(id int, client net.Conn, recvChan chan []byte, broadcastChan chan msg) {
	clientChan := make(chan []byte)
	quit := make(chan bool)
	go readClient(client, clientChan, quit)
	defer client.Close()
	for {
		select {
		case sendMsg := <-clientChan:
			if cmdIndex := strings.Index(string(sendMsg), ":"); cmdIndex == -1 {
				broadcastChan <- msg{id, sendMsg}
			} else {
				switch string(sendMsg)[:cmdIndex] {
				case "whoami":
					whoamiMsg := []byte("chitter: " + strconv.Itoa(id) + "\n")
					client.Write(whoamiMsg)
				case "all":
					broadcastChan <- msg{id, []byte(string(sendMsg)[cmdIndex+1:])}
				default:
					fmt.Printf("Unrecognized command from client %d\n", id)
				}
			}
		case rcvdMsg := <-recvChan:
			client.Write(rcvdMsg)
		case <-quit:
			fmt.Printf("Closing client %d\n", id)
			return
		}
	}
}

func readClient(client net.Conn, clientChan chan []byte, quitChan chan bool) {
	reader := bufio.NewReader(client)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf("Client ended the connection\n")
			} else {
				fmt.Println("Server error reading stream: " + err.Error())
			}
			quitChan <- true
			return
		}
		clientChan <- line
	}
}
