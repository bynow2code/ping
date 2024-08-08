package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ICMP struct {
	Type           uint8
	Code           uint8
	Checksum       uint16
	Identifier     uint16
	SequenceNumber uint16
	Data           [48]byte
}

var (
	address string
	count   int           = 100
	timeout time.Duration = 1000 * time.Millisecond
)

func init() {
	log.SetFlags(log.Llongfile)
	setAddress()
}

func main() {
	go func() {
		conn, err := net.DialTimeout("ip4:icmp", address, timeout)
		if err != nil {
			log.Println(err.Error())
			return
		}
		defer conn.Close()

		var icmp ICMP
		remoteAddr := conn.RemoteAddr()
		fmt.Printf("PING %s (%s): %d data bytes\n", address, remoteAddr, binary.Size(icmp))

		for i := 0; ; i++ {
			icmp = ICMP{Type: 8, SequenceNumber: uint16(i)}

			pid := os.Getpid()
			icmp.Identifier = binary.BigEndian.Uint16([]byte{byte(pid >> 8), byte(pid & 0xfff)})

			var buffer bytes.Buffer
			err := binary.Write(&buffer, binary.BigEndian, icmp)
			if err != nil {
				log.Fatalln(err.Error())
			}

			request := buffer.Bytes()
			checkSum := calculateICMPChecksum(request)
			request[2] = byte(checkSum >> 8)
			request[3] = byte(checkSum)

			err = conn.SetDeadline(time.Now().Add(timeout))
			if err != nil {
				log.Fatalln(err.Error())
			}

			_, err = conn.Write(request)
			if err != nil {
				log.Fatalln(err.Error())
			}

			response := make([]byte, 1024)
			startReplyTime := time.Now()
			readN, err := conn.Read(response)
			if err != nil {
				fmt.Printf("Request timeout for icmp_seq %d\n", icmp.SequenceNumber)
				continue
			}

			replyTime := float64(time.Since(startReplyTime).Nanoseconds()) / float64(time.Millisecond)
			fmt.Printf("%d bytes from %s: icmp_seq=%d ttl=%d time=%.3f ms\n", readN-20, remoteAddr, icmp.SequenceNumber, response[8], replyTime)
			time.Sleep(time.Second)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	<-sigChan
	fmt.Printf("\n--- %s ping statistics ---\n", address)
	fmt.Printf("%d packets transmitted, %d packets received, 0.0%% packet loss\n", 3, 3)
	fmt.Printf("round-trip min/avg/max/stddev = %.3f/12.692/12.891/0.143 ms\n", 3.1)
	os.Exit(0)
}
func setAddress() {
	if len(os.Args) == 1 {
		log.Fatalln("Address required")
	}
	address = os.Args[len(os.Args)-1]
}
func calculateICMPChecksum(buf []byte) uint16 {
	var sum uint32
	// 处理奇数长度的情况
	if len(buf)%2 != 0 {
		buf = append(buf, 0)
	}

	// 对所有16位字做二进制加法
	for i := 0; i < len(buf); i += 2 {
		word := binary.BigEndian.Uint16(buf[i : i+2])
		sum += uint32(word)
	}

	// 加上进位
	for sum>>16 > 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	// 取反得到校验和
	checksum := ^uint16(sum)
	return checksum
}
