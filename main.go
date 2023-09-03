package main

import (
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	_ "sync"
	"time"
)

func verify_ip_input(ip_addr string) {
	re := regexp.MustCompile(`(?m)^((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}\/(\d\d|\d)$`)
	match := re.MatchString(ip_addr)
	if !match {
		panic("Please input a valid ipv4 address with range\nEx. 127.0.0.1/8")
	}
}

func get_ip_from_int(ip int) net.IPAddr {
	var res [4]uint8
	for i := 0; i < 4; i++ {
		res[i] = uint8(((ip & (0xFF << (i * 8))) >> (i * 8)))
	}
	ip_addr := net.IPAddr{IP: net.IPv4(res[3], res[2], res[1], res[0])}
	return ip_addr
}

func Ping(ip *net.IPAddr, listen_address string) (bool, error) {
	c, err := icmp.ListenPacket("ip4:icmp", listen_address)
	if err != nil {
		return false, fmt.Errorf("1")
	}
	defer c.Close()

	icmp_message := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte(""),
		},
	}

	binary, err := icmp_message.Marshal(nil)
	if err != nil {
		return false, fmt.Errorf("2")
	}

	n, err := c.WriteTo(binary, ip)
	if err != nil {
		return false, fmt.Errorf("3")
	} else if n != len(binary) {
		return false, fmt.Errorf("got %v; want %v", n, len(binary))
	}

	reply := make([]byte, 1500)
	err = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return false, fmt.Errorf("4")
	}
	n, peer, err := c.ReadFrom(reply)
	if err != nil {
		return false, err
	}

	rm, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return false, fmt.Errorf("6")
	}
	switch rm.Type {
	case ipv4.ICMPTypeEchoReply:
		return true, nil
	default:
		return true, fmt.Errorf("got %+v from %v; want echo reply", rm, peer)
	}
}

func get_hilbert_coordinates(ip_number, size int) (int, int) {
	var directionX, directionY, resX, resY int
	resX, resY = 0, 0
	for i := 1; i < size; i *= 2 {
		directionX = 1 & (ip_number / 2)
		directionY = 1 & (ip_number ^ directionX)
		if directionY == 0 {
			if directionX == 1 {
				resX = i - 1 - resX
				resY = i - 1 - resY
			}
			resX, resY = resY, resX
		}
		resX += i * directionX
		resY += i * directionY
		ip_number /= 4
	}
	return resX, resY
}

type address_info struct {
	ip_address net.IPAddr
	number     int
}

type response_info struct {
	response bool
	number   int
}

func main() {
	args := os.Args
	if len(args) == 1 {
		panic("Please input a valid ipv4 address with range\nEx. 127.0.0.1/8")
	}

	verify_ip_input(args[1])

	address_parts := strings.Split(args[1], "/")
	temp, _ := strconv.ParseInt(address_parts[1], 10, 32)
	span := int(temp)
	if span > 32 || span <= 1 {
		panic("Please input a range greater than 1 and less than 32")
	}

	octet_list := strings.Split(address_parts[0], ".")
	num_ip_addr := 0
	shift := 0
	for i := len(octet_list) - 1; i >= 0; i-- {
		num_octet, _ := strconv.ParseInt(octet_list[i], 10, 32)
		num_ip_addr |= int(num_octet) << (shift * 8)
		shift++
	}

	num_ip_addr >>= span
	num_ip_addr <<= span

	size := 2
	for i := 1; i < span; i++ {
		size *= 2
	}

	address_channel := make(chan address_info, 10)
	response_channel := make(chan response_info, 10)

	address_channel <- address_info{ip_address: get_ip_from_int(num_ip_addr), number: 0}

	for i := 0; i < 20; i++ {
		go func() {
			for {
				ip_address_info := <-address_channel
				resp, err := Ping(&ip_address_info.ip_address, "0.0.0.0")
				for err == fmt.Errorf("read ipv4 0.0.0.0: i/o timeout") {
					if err != nil {
						fmt.Println(err)
					}
					resp, err = Ping(&ip_address_info.ip_address, "0.0.0.0")
				}
				response_channel <- response_info{response: resp, number: ip_address_info.number}
			}
		}()
	}

	go func() {
		number := 1
		size := size
		for number < size {
			address_channel <- address_info{ip_address: get_ip_from_int(num_ip_addr + number), number: number}
			number++
		}
	}()

	upLeft := image.Point{0, 0}
	lowRight := image.Point{span * 2, span * 2}
	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})

	cyan := color.RGBA{100, 200, 200, 0xff}

	response_count := 0
	for {
		resp := <-response_channel
		fmt.Println(resp)
		x, y := get_hilbert_coordinates(resp.number, size)
		if resp.response {
			img.Set(x, y, color.White)
		} else {
			img.Set(x, y, cyan)
		}
		response_count++
		if response_count == size {
			fmt.Println(response_count)
			break
		}
	}

	f, _ := os.Create("image.png")
	png.Encode(f, img)
}
