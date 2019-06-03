package main

import (
	"flag"
	"github.com/fatih/color"
	"github.com/methanduck/GO/RelaySVR"
	"log"
	"os/exec"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	Server_port := flag.String("port", RelaySVR.Service_port, "Server Port")
	Server_Addr := flag.String("addr", "127.0.0.1", "Server Addr")

	var addrModified []byte
	if *Server_Addr == "" {
		addr, err := exec.Command("/bin/sh", "-c", "awk 'END{print $1}' /etc/hosts").Output()
		if err != nil {
			red := color.New(color.FgRed).SprintFunc()
			log.Println(red("ERROR!! Relay server failed to get address or port!!"))
		}
		addrModified = addr[:len(addr)-1]
	}
	if err := run(string(addrModified), *Server_port); err != nil {
		red := color.New(color.FgRed).SprintFunc()
		log.Panic(red("Stop running" + err.Error()))
	}
}

func run(address string, port string) error {
	server := new(RelaySVR.Server)
	if err := server.Start(address, port); err != nil {
		return err
	}
	return nil
}
