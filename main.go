package main

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
	"flag"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
	"os"
)

var port uint;


func init() {
	const (
		Port = "请输入监听端口"
	)
	flag.UintVar(&port, "port", 553, Port)
	flag.Parse()
}

func main() {
	udpAddr, err := net.ResolveUDPAddr("udp", ":" + strconv.Itoa(int(port)))
	if err != nil {
		fmt.Println(err)
		return
	}

	socket, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer socket.Close()

	fd, err := os.OpenFile("querylog-" + strconv.Itoa(int(port)) + ".txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fd.Close()

	messageC := make(chan string, 5)
	fileC := make(chan string, 5)
	defer close(fileC)
	defer close(messageC)
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context){
		for {
			select {
			case msg := <- messageC:
				log.Println(msg)
			case content := <- fileC:
				fd.Write([]byte(content))
			case <- ctx.Done():
				return
			}
		}
	}(ctx)
	defer cancel()

	messageC <- fmt.Sprintf("listen dns packet on %s", socket.LocalAddr())
	buffer := make([]byte, 1024)
	for {
		count, client, err := socket.ReadFrom(buffer)
		if err != nil {
			messageC <- err.Error()
			continue
		}
		messageC <- fmt.Sprintf("receive %d bytes from %s",count, client)
		packet := gopacket.NewPacket(buffer, layers.LayerTypeDNS, gopacket.Default).Layer(layers.LayerTypeDNS)
		if packet == nil {
			messageC <- fmt.Sprintf("this packet is not a dns packet")
			continue
		}
		query := packet.(*layers.DNS)
		str := ""
		for _, question := range query.Questions {
			str += fmt.Sprintf("[%s]: client:%s, domain: %s, type: %s, class: %s\n", time.Now().Format("2006-01-02 15:03:04"), client, string(question.Name),question.Type, question.Class)
		}
		fileC <- str
		response := &layers.DNS{
			ID: query.ID,
			QR: true,
			OpCode: layers.DNSOpCodeQuery,
            AA: false,
            TC: false,
            RD: true,
            RA: true,
            ResponseCode: layers.DNSResponseCodeNoErr,
            QDCount: query.QDCount,
            ANCount: query.QDCount,
            NSCount: 0,
            ARCount: 0,
            Questions: query.Questions,
            Answers: query.Answers,
            Authorities: query.Authorities,
            Additionals: query.Additionals,
		}
		buf := gopacket.NewSerializeBuffer()
		err = gopacket.SerializeLayers(buf,gopacket.SerializeOptions{},response)
		if err != nil {
			messageC <- err.Error()
			continue
		}
		socket.SetWriteDeadline(time.Now().Add(30 * time.Second))
		_, err = socket.WriteTo(buf.Bytes(),client)
		if err != nil {
			messageC <- err.Error()
		}
	}
}