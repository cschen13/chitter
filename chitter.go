// Chris Chen, csc404
// Project 0
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
	cmd  string
	line string
}

func main() {
	isClient := flag.Bool("c", false, "Connect as a client")
	flag.Parse()
	if len(flag.Args()) > 0 && len(flag.Args()) < 3 {
		if *isClient {
			if len(flag.Args()) == 2 {
				serverIP := flag.Args()[0]
				serverPort := flag.Args()[1]
				conn, err := net.Dial("tcp", serverIP+":"+serverPort)
				if err != nil {
					println("Failed to connect to server: " + err.Error())
					return
				}
				fmt.Printf("Connection established: %v <-> %v\n", conn.LocalAddr(), conn.RemoteAddr())
				fmt.Println("Use Ctrl+C to disconnect from the server")
				chatClient(conn)
			} else {
				fmt.Println("usage: go run chitter.go [-c <server_ip>] <port_number>")
			}
		} else { //Start a server
			if len(flag.Args()) == 1 {
				serverPort := flag.Args()[0]
				chatServer(serverPort)
			} else {
				fmt.Println("usage: go run chitter.go [-c <server_ip>] <port_number>")
			}
		}
	} else {
		fmt.Println("usage: go run chitter.go [-c <server_ip>] <port_number>")
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
	fmt.Println("Use Ctrl+C to close the server")
	id, idChannelMap := 0, make(map[int]chan string)
	newConnChan, sendChan := make(chan net.Conn), make(chan Msg)
	go acceptConnections(server, newConnChan)
	for {
		select {
		case client := <-newConnChan:
			idChannelMap[id] = make(chan string)
			fmt.Printf("New client %d\n", id)
			go handleClient(id, client, idChannelMap[id], sendChan)
			id++
		case msg := <-sendChan:
			handleMessage(msg, idChannelMap)
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

func handleMessage(msg Msg, idChannelMap map[int]chan string) {
	senderId := strconv.Itoa(msg.id)
	switch msg.cmd {
	case "all":
		for _, channel := range idChannelMap {
			channel <- senderId + ": " + msg.line
		}
	case "whoami":
		idChannelMap[msg.id] <- "chitter: " + senderId + "\n"
	case "close":
		close(idChannelMap[msg.id])
		delete(idChannelMap, msg.id)
	default:
		if recipientId, err := strconv.Atoi(msg.cmd); err == nil {
			if channel, ok := idChannelMap[recipientId]; ok {
				channel <- senderId + ": " + msg.line
				fmt.Printf("Sent private message from %d to %d\n", msg.id, recipientId)
			} else {
				// idChannelMap[msg.id] <- "chitter: No active user with ID " + msg.cmd + "\n"
				fmt.Printf("chitter: No active user with ID %s\n", msg.cmd)
			}
		} else {
			fmt.Printf("Unrecognized command \"%s\" from client %d\n", msg.cmd, msg.id)
		}
	}
}

func handleClient(id int, client net.Conn, recvChan chan string, sendChan chan Msg) {
	clientChan, quit := make(chan []byte), make(chan bool)
	go readSelect(bufio.NewReader(client), clientChan, quit)
	defer client.Close()
	for {
		select {
		case sendMsg := <-clientChan:
			if cmdIndex := strings.Index(string(sendMsg), ":"); cmdIndex == -1 {
				sendChan <- Msg{id, "all", strings.TrimLeft(string(sendMsg), " \t")}
			} else {
				cmd := strings.TrimSpace(string(sendMsg)[:cmdIndex])
				line := strings.TrimLeft(string(sendMsg)[cmdIndex+1:], " \t")
				sendChan <- Msg{id, cmd, line}
			}
		case rcvdStr := <-recvChan:
			fmt.Fprintf(client, rcvdStr)
		case <-quit:
			fmt.Printf("Client %d closed\n", id)
			//Request client entry in idChannelMap be removed
			sendChan <- Msg{id, "close", ""}
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
