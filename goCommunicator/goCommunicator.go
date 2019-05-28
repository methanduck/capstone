package goCommunicator

import (
	"encoding/json"
	"flag"
	"github.com/fatih/color"
	"github.com/methanduck/capstone/InteractiveSocket"
	"log"
	"net/rpc"
	"strconv"
)

func main() {
	arguments := flag.Args()
	if err := RPC(arguments); err != nil {
		log.Fatal(color.RedString("ERR :: goCommunication : failed to send data "))
	} else {
		log.Println(color.GreenString("INFO :: goCommunication : command complete"))
	}
}

func RPC(data []string) error {
	client, err := rpc.Dial("tcp", "localhost:"+InteractiveSocket.RPCLISTENINGPORT)
	if err != nil {
		return err
	}

	if marshalledData, err := Interpreter(data); err != nil {
		return err
	} else {
		var reply bool
		if err := client.Call("remoteprocedure.RPC_TOAPP", marshalledData, reply); err != nil {
			return err
		}
	}
	return nil
}
func Interpreter(data []string) ([]byte, error) {
	var err error
	nodeData := InteractiveSocket.Node{}
	nodeData.Temp_IN, err = strconv.Atoi(data[0])
	nodeData.Temp_OUT, err = strconv.Atoi(data[1])
	nodeData.Humidity_IN, err = strconv.Atoi(data[2])
	nodeData.Humidity_OUT, err = strconv.Atoi(data[3])
	nodeData.Gas, err = strconv.Atoi(data[4])
	nodeData.Smoke, err = strconv.ParseBool(data[5])
	nodeData.Light, err = strconv.Atoi(data[6])
	nodeData.Rain, err = strconv.ParseBool(data[7])
	nodeData.ModeAuto, err = strconv.ParseBool(data[8])
	nodeData.Dust, err = strconv.Atoi(data[9])

	marshalledData, err := json.Marshal(nodeData)
	if err != nil {
		return nil, err
	} else {
		return marshalledData, nil
	}
}
