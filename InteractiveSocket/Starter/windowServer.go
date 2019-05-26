package main

import (
	"flag"
	"github.com/fatih/color"
	"log"
	"os"
	"os/exec"
	"runtime"
)
import (
	"github.com/methanduck/GO/InteractiveSocket"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	address, port, path, command := param()
	if err := run(*address, *port, *path, *command); err != nil {
		log.Panic(color.RedString("Server stopped :" + err.Error()))
	}

}
func run(address string, port string, path string, command string) error {
	localServer := InteractiveSocket.Window{}
	err := localServer.Start(address, port, path)
	return err
}
func param() (address *string, port *string, path *string, command *string) {
	address = flag.String("address", "127.0.0.1", "set window address")
	port = flag.String("port", "6866", "set window port")
	path = flag.String("pythonpath", "./", "set python command path")
	command = flag.String("command", "", "set python command")
	flag.Parse()

	if *port == "127.0.0.1" {
		if envPort := os.Getenv("port"); envPort != "" {
			*port = envPort
		}
	}
	if *address == "" {
		if envAddress := os.Getenv("address"); envAddress != "" {
			*address = envAddress
		}
	}

	if *path == "" {
		if envPath := os.Getenv("pythonpath"); envPath != "" {
			*path = envPath
		}
	}

	if *command == "" {
		if envCommand := os.Getenv("command"); envCommand != "" {
			*command = envCommand
		}
	}

	if *address == "" {
		addr, err := exec.Command("/bin/sh", "-c", "awk 'END{print $1}' /etc/hosts").Output()
		if err != nil {
			color.Set(color.FgRed)
			defer color.Unset()
			log.Println("ERROR!! Socket server failed to get address or port!!")
			log.Panic("Aborting initialize" + err.Error())
		}
		addrModified := addr[:len(addr)-1]
		*address = string(addrModified)
	}
	return
}
