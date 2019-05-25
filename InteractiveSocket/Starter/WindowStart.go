package main

import (
	"flag"
	"github.com/fatih/color"
	"log"
	"os/exec"
	//	"flag"
	//	"github.com/fatih/color"
	//	"log"
	//	"os"
	//	"os/exec"
	"os"
	"runtime"
)
import (
	"github.com/methanduck/GO/InteractiveSocket"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	address := flag.String("address", "127.0.0.1", "set window address")
	port := flag.String("port", "6866", "set window port")
	path := flag.String("pythonpath", "./", "set python command path")
	flag.Parse()
	if *port == "" {
		*port = os.Getenv("port")
	}
	if *address == "" {
		*address = os.Getenv("address")
	}
	if *path == "" {
		*path = os.Getenv("pythonpath")
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
		if err := run(string(addrModified), *port, *path); err != nil {
			red := color.New(color.FgRed).SprintFunc()
			log.Panic(red("Stop running" + err.Error()))
		}
	}
	flag.Parse()

	_ = run(*address, *port, "python "+*path)

}
func run(address string, port string, path string) error {
	localServer := InteractiveSocket.Window{}
	err := localServer.Start(address, port, path)
	return err
}
