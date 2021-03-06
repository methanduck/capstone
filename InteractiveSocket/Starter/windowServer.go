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
	"github.com/methanduck/capstone/InteractiveSocket"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	address, port, path, filename, pythonclient, pythoninfo := param()
	if err := run(*address, *port, *path, *filename, *pythonclient, *pythoninfo); err != nil {
		log.Panic(color.RedString("Server stopped :" + err.Error()))
	}
}
func run(address string, port string, path string, filename string, pythonclient string, pythoninfo string) error {
	localServer := InteractiveSocket.Window{}
	err := localServer.Start(address, port, path, filename, pythonclient, pythoninfo)
	return err
}
func param() (address *string, port *string, path *string, filename *string, pythonclient *string, pythoninfo *string) {
	address = flag.String("address", "127.0.0.1", "set window address")
	port = flag.String("port", "6866", "set window port")
	path = flag.String("pythonpath", "./", "set python command path")
	filename = flag.String("filename", "", "set python filename")
	pythonclient = flag.String("pyport", "", "set python listening server port between python - golang")
	pythoninfo = flag.String("pyinfo", "1234", "set python listening server port (info) between python - golang")
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
	} else {
		tmpPath := *path
		if tmpPath[len(tmpPath)-1:] != "/" {
			*path = *path + "/"
		}
	}

	if *filename == "" {
		if envFilename := os.Getenv("filename"); envFilename != "" {
			*filename = envFilename
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
