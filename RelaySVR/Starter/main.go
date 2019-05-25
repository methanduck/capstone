package main

import (
	"github.com/fatih/color"
	"github.com/methanduck/GO/RelaySVR"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/* change flag to os.Getenv
	Server_port := flag.String("port", RelaySVR.Service_port, "Server Port")
	Server_Addr := flag.String("addr", "", "Server Addr")
	*/
	address := os.Getenv("serverwindow")
	port := os.Getenv("serverport")
	if address == "" {
		addr, err := exec.Command("/bin/sh", "-c", "awk 'END{print $1}' /etc/hosts").Output()
		if err != nil {
			red := color.New(color.FgRed).SprintFunc()
			log.Println(red("ERROR!! Relay server failed to get address or port!!"))
			log.Panic(red("Aborting initialize" + err.Error()))

		}
		addrModified := addr[:len(addr)-1]
		if err := run(string(addrModified), port); err != nil {
			red := color.New(color.FgRed).SprintFunc()
			log.Panic(red("Stop running" + err.Error()))
		}
	}
}

func run(address string, port string) error {
	server := new(RelaySVR.Server)
	if err := server.Start(address, port); err != nil {
		return err
	}
	return nil
}
