package InteractiveSocket

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

//고정 변수
const (
	RPCLISTENINGPORT = "66866"
	//서버 포트
	SVRLISTENINGPORT = "6866"
	//중계서버 IP
	RELAYSVRIPADDR = "127.0.0.1:6866"
)

type Window struct {
	PInfo      *log.Logger
	PErr       *log.Logger
	svrInfo    *Node
	Available  *sync.Mutex
	FAvailable *sync.Mutex
	quitSIGNAL chan string
	python     *python
	ipc        *remoteprocedure
}
type python struct {
	path     string
	filename string
}
type remoteprocedure struct {
	window *Window
}

var androidWaiting *list.List

//VALIDATION 성공 : "LOGEDIN" 실패 : "ERR"
//각 인자의 구분자 ";"
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  1. 초기화 되지 않은 노드에 접속 시 Initialized = false 전송 (json)																					//
//  2. 안드로이드는 false 수신 시 설정 값을 json으로 전송 이 때 json Oper 항목에 창문 수행 명령이 있을경우 바로 수행											//
//  3. 수행명령이 존재하지 않을 경우 값을 파일로 쓰고 안드로이드로 "OK" 를 json 으로 전송함																	//
//																																					//
// 1-1. 초기화 된 노드에 접속시 Initialized = true 전송 (json)																							//
// 2-1. 안드로이드는 true 수신 시 창문에 validation과 동시에 명령을 수행시키기 위해 창문으로 수행 명령 (JSON) Oper항목에 명령을 담아 전송							//
// 2-2. 창문에서 validation 수행 실패 시 "ERR"를 json으로 전송하고 validation 수행 성공 시 Operations 로 넘어가 창문을 조작									//
// 3-1. 수행이 종료되면 "OK"를 json으로 전송																											//
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (win *Window) afterConnected(Android net.Conn) {

	switch win.svrInfo.Initialized {
	//초기화가 되어 있는 경우
	case true:
		_ = COMM_SENDJSON(*win, Android)
		//데이터 수신
		recvData, err := COMM_RECVJSON(Android)
		if err != nil {
			win.PErr.Println(color.RedString("Failed to receive json"))
		}
		//자격증명 수행
		if err := win.svrInfo.HashValidation(recvData.PassWord, MODE_VALIDATION); err != nil {
			win.PErr.Println("Login failed (Android :" + Android.RemoteAddr().String() + ")")
			win.COMM_ACK(COMM_FAIL, Android)
		} else {
			win.PInfo.Println("Login succeeded (Android :" + Android.RemoteAddr().String() + ")")
			//자격증명 성공시 명령 수행
			win.Operation(recvData, Android)
		}

	//초기화가 되어 있지 않는 경우
	case false:
		win.COMM_ACK("Configuration require", Android)
		recvData, err := COMM_RECVJSON(Android)
		if err != nil {
			win.PErr.Println(color.RedString("Failed to receive json"))
		}

		//패스워드 없이 초기 창문확인을 위한 명령이 들어올때 수행합니다.
		if recvData.PassWord == "" {
			if recvData.Oper != "" {
				win.PInfo.Println("Operate without initialize (Android :" + Android.RemoteAddr().String() + ")")
				win.Operation(recvData, Android)
			}
		} else {
			//패스워드가 제공되면서 창문 명령이 전달될 경우를 처리합니다.
			//수신된 비밀번호를 설정하되 다중 입력이 들어올 경우 race condition이 발생하므로
			// 1. 가장 먼저 자료를 송신한 app
			// 		+ 파일 설정의 lock을 획득 및 램에 적재된 node객체의 initialized = true로 설정 다른 app의 접근을 제한
			//		+ 송신한 객체에 Oper 자료가 존재하면 해당 명령을 창문에 수행함
			// 2. 이후 늦게 자료를 송신한 app
			// 		+ 파일 설정의 lock을 획득하기 이전 node객체의 initialized = true로 인해 FAIL 수신
			//		+ app입장에서는 연결이 종료되고 다시 창문에 접근해야함(자격증명)
			if win.svrInfo.Initialized {
				win.PErr.Println(color.RedString("Initialize failure (Android :" + Android.RemoteAddr().String() + ")"))
				win.COMM_ACK(COMM_FAIL, Android)
				return
			} else {
				win.PInfo.Println(color.GreenString("Commencing data flush (from Android :" + Android.RemoteAddr().String() + ")"))
				win.FAvailable.Lock()
				win.svrInfo.DATA_INITIALIZER(recvData, true)
				if err := win.svrInfo.FILE_FLUSH(); err != nil {
					win.PErr.Println(err)
				}
				win.FAvailable.Unlock()
				win.svrInfo.PrintData()
				win.COMM_ACK(COMM_SUCCESS, Android)
				if recvData.Oper != "" {
					win.Operation(recvData, Android)
				}
			}
		}
	}

	win.PInfo.Println("Connection terminated with :" + Android.RemoteAddr().String())
	_ = Android.Close()
}

func (win *Window) updateToRelaySVR() {
	win.quitSIGNAL = make(chan string, 1)
	green := color.New(color.BgGreen).SprintfFunc()
	win.PInfo.Println(green("Relay server communication is now in effect"))
	go func() {
	loop:
		for {
			sig := <-win.quitSIGNAL
			switch sig {
			case "stop":
				break loop

			default:
				conn, err := net.Dial("TCP", RELAYSVRIPADDR)
				if err != nil {
					win.PErr.Println("connection err with relaySVR: " + err.Error())
				}
				time.Sleep(2 * time.Second)
				_ = COMM_SENDJSON(&Node{Oper: STATE_ONLINE}, conn)
				inNode, err := COMM_RECVJSON(conn)
				if err == nil {
					win.RelayOperation(inNode, conn)
				}
			}
		}
	}()
}

func (win *Window) close_UpdateToRerlaySVR() {
	green := color.New(color.BgGreen).SprintfFunc()
	win.PInfo.Println(green("communication is now ineffect"))
	win.quitSIGNAL <- "stop"
}

func (win *Window) RelayOperation(reqNode Node, remote net.Conn) {
	switch reqNode.Ack {
	case COMM_SUCCESS:
		win.PInfo.Println(color.GreenString("Successfully update online to relay server"))
	default:
		win.Operation(reqNode, remote)
	}
}

func (win *Window) SocketOperation(Android net.Conn) {
	for {
		AndroidNode, err := COMM_RECVJSON(Android)
		if err != nil {
			break
		}
		win.Operation(AndroidNode, Android)
	}

}

//창문 명령
func (win *Window) Operation(order Node, android net.Conn) {
	win.Available.Lock()
	switch order.Oper {
	case OPERATION_OPEN:
		if err := win.PYTHON_USER_CONF("window", "1"); err != nil {
			win.PErr.Println(color.RedString("failed to run command : OPEN (err code :" + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			win.PInfo.Println("executed command : OPEN")
			win.COMM_ACK(COMM_SUCCESS, android)
		}

	case OPERATION_CLOSE:
		if _, err := exec.Command("/bin/sh", "-c", win.python.path+win.python.filename).Output(); err != nil {
			win.PErr.Println(color.RedString("failed to run command : CLOSE"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			win.PInfo.Println("executed command : CLOSE")
			win.COMM_ACK(COMM_SUCCESS, android)
		}

	case OPERATION_INFORMATION:

		/*
			if data, err := exec.Command("/bin/sh", "-c", win.python.path+win.python.filename).Output(); err != nil {
				win.PErr.Println(color.RedString("failed to run command : INFO ( check error code below)"))
				win.PErr.Println(err.Error())
				win.COMM_ACK(COMM_FAIL, android)
			} else {
				win.PInfo.Println("executed command : INFO")
				win.svrInfo.Ack = COMM_SUCCESS
				if err := win.Interpreter(string(data)); err != nil {
					win.PErr.Println(color.RedString("failed to unmarshalling data"))
					win.COMM_ACK(COMM_FAIL, android)
				} else {
					_ = COMM_SENDJSON(*win.svrInfo, android)
					win.PInfo.Println("")
				}
			}*/

	case OPERATION_MODEAUTO:
		if err := win.PYTHON_USER_CONF("auto", "22"); err != nil {
			win.PInfo.Println(color.RedString("failed to run command : WINDOW_MODE_AUTO (err code :" + err.Error() + ")"))
		}
		win.svrInfo.ModeAuto = order.ModeAuto
		if win.svrInfo.ModeAuto {
			win.PInfo.Println("executed command : WINDOW_MODE_AUTO=TRUE")
		} else {
			win.PInfo.Println("executed command : WINDOW_MODE_AUTO=FALSE")
		}
		win.COMM_ACK(COMM_SUCCESS, android)
	case OPERATION_PROXY:
		win.svrInfo.ModeProxy = order.ModeProxy
		if win.svrInfo.ModeProxy {
			win.updateToRelaySVR()
		} else {
			win.close_UpdateToRerlaySVR()
		}
		win.COMM_ACK(COMM_SUCCESS, android)
	default:
		win.PErr.Println(color.RedString("received not compatible command (OPER)"))
		win.COMM_ACK(COMM_FAIL, android)
	}
	win.Available.Unlock()

}

//응답 송신
func (win *Window) COMM_ACK(result string, android net.Conn) {
	win.svrInfo.Ack = result
	res, _ := json.Marshal(win.svrInfo)
	_, _ = android.Write(res)
}

//**************************deprecated****************************************
//PYTHON : 창문 여닫이와 필름 조종위한 파일 상태 Reader
//command
// window : 창문		film : 필름		auto : 자동모
func (win *Window) PYTHON_USER_CONF(target string, command string) error {
	byteFileData := make([]byte, 300)
	if _, err := os.Stat(win.python.path + win.python.filename); err != nil {
		return err
	} else {
		file, err := os.OpenFile(win.python.path+win.python.filename, os.O_RDWR, 755)
		defer file.Close()
		if err != nil {
			return err
		} else {
			if _, err = file.ReadAt(byteFileData, 0); err != io.EOF {
				fmt.Println(string(byteFileData))
				return err
			}
			switch target {
			case "window":
				stringFileData := string(byteFileData)
				fmt.Println(stringFileData)
				stringFileData = stringFileData[1:]
				stringFileData = command + stringFileData
				if _, err = file.WriteAt([]byte(stringFileData), 0); err != nil {
					return err
				}
			case "film":
				if _, err = file.WriteAt([]byte(command), 1); err != nil {
					return err
				}

			case "auto":
				if _, err = file.WriteAt([]byte(command), 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

//****************************************************************************

//JSON파일 전송
func COMM_SENDJSON(data interface{}, android net.Conn) error {
	marshalledData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("COMM_SVR : SocketSVR Marshalled failed")
	}
	_, _ = android.Write(marshalledData)
	return nil
}

//JSON파일 수신
func COMM_RECVJSON(android net.Conn) (res Node, err error) {
	inStream := make([]byte, 4096)
	tmp := Node{}
	n, err := android.Read(inStream)
	if err != nil {
		return res, fmt.Errorf("COMM_SVR : SocketSVR failed to receive message")
	}
	err = json.Unmarshal(inStream[:n], &tmp)
	if err != nil {
		return res, fmt.Errorf("COMM_SVR : SocketSVR failed to Unmarshaling data stream")
	}
	return tmp, nil
}

//OS 명령 실행
func (win *Window) EXEC_COMMAND(comm string) string {
	out, err := exec.Command("/bin/bash", "-c", comm).Output()
	if err != nil {
		win.PInfo.Println("failed to run command")
	}
	return string(out)
}

//프로그램 시작부
func (win *Window) Start(address string, port string, path string, filename string, pythonpath string) error {
	//구조체 객체 선언
	win.svrInfo = &Node{}
	win.python = &python{}
	win.ipc = &remoteprocedure{}
	win.python.filename = filename
	win.python.path = path
	win.PErr = log.New(os.Stdout, color.RedString("ERR :: Socket server: "), log.LstdFlags)
	win.PInfo = log.New(os.Stdout, "INFO :: Socket server :", log.LstdFlags)
	win.Available = new(sync.Mutex)
	win.FAvailable = new(sync.Mutex)
	win.quitSIGNAL = make(chan string)
	androidWaiting = list.New()
	win.ipc.window = win
	//IPC 위한 스레드
	go func() {
		if err := win.ipc.Ipc_Start(pythonpath); err != nil {
			win.PErr.Fatal(color.RedString("IPC server ::[ERR] failed to init ipc server,(err code :" + err.Error() + " Abort."))
		}
	}()

	if err := win.svrInfo.FILE_INITIALIZE(); err != nil {
		win.PErr.Println(err)
	} else {
		win.PInfo.Println(color.BlueString("[OK] File loaded"))
	}
	//서버 리스닝 시작부
	Android, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		win.PErr.Fatal("[ERR] failed to open socket ( address :" + address + " port :" + port + "), (err code :" + err.Error() + ", Abort")
		return err
	} else {
		win.PInfo.Println(color.BlueString("[OK] initialized = " + address + ":" + port))
		win.PInfo.Println("#############################Currently configured data################################")
		win.svrInfo.PrintData()
		win.PInfo.Println("######################################################################################")
		win.PInfo.Println(color.BlueString("[OK] configured parameter"))
		win.PInfo.Println("#############################Currently configured parameter################################")
		win.PInfo.Println("Python path : " + win.python.path)
		win.PInfo.Println("Python file name : " + win.python.filename)
		win.PInfo.Println("###########################################################################################")
	}

	defer func() {
		err := Android.Close()
		if err != nil {
			win.PErr.Println("terminated abnormaly" + Android.Addr().String())
		}
	}()

	for {
		connect, err := Android.Accept()
		if err != nil {
			win.PErr.Println("failed to connect TCP with :" + connect.RemoteAddr().String())
		} else {
			win.PInfo.Println("successfully TCP connected with :" + connect.RemoteAddr().String())
			//start go routine
			go win.afterConnected(connect)
		}
		defer func() {
			err := connect.Close()
			if err != nil {
				win.PErr.Println("connection terminated abnormaly with client :" + Android.Addr().String())
			}
		}()
	}
}

//센서 데이터 해석
func (win *Window) Interpreter(data string) (err error) {
	result := strings.Split(data, DELIMITER)
	win.svrInfo.Smoke, err = strconv.ParseBool(result[0])
	win.svrInfo.Rain, err = strconv.ParseBool(result[1])
	win.svrInfo.Light, err = strconv.Atoi(result[2])
	win.svrInfo.Motion, err = strconv.ParseBool(result[3])
	win.svrInfo.Humidity_IN, err = strconv.Atoi(result[4])
	win.svrInfo.Temp_IN, err = strconv.Atoi(result[5])
	win.svrInfo.Gas, err = strconv.Atoi(result[6])
	win.svrInfo.Dust, err = strconv.Atoi(result[7])
	win.svrInfo.Humidity_OUT, err = strconv.Atoi(result[8])
	win.svrInfo.Temp_OUT, err = strconv.Atoi(result[9])
	return
}

//ipc 리스너
func (remote *remoteprocedure) Ipc_Start(pythonpath string) error {
	listener, err := net.Listen("tcp", "0.0.0.0:"+pythonpath)
	if err != nil {
		return err
	} else {
		remote.window.PInfo.Println(color.BlueString("IPC server :: [OK] initialized = " + listener.Addr().String()))
	}

	for {
		connect, err := listener.Accept()
		if err != nil {
			remote.window.PErr.Println(color.RedString(" IPC server :: failed to connect TCP with : " + connect.RemoteAddr().String()))
		} else {
			remote.window.PInfo.Println(" IPC server :: successfully TCP connected with : " + connect.RemoteAddr().String())
			go remote.ipc_Procedure(connect)
		}
		defer func() {
			if err := connect.Close(); err != nil {
				remote.window.PErr.Println("IPC server :: connection terminated abnormaly with : " + connect.RemoteAddr().String())
			}
		}()
	}

}

//ipc 프로시저
func (remote *remoteprocedure) ipc_Procedure(conn net.Conn) {
	data := make([]byte, 500)
	size, err := conn.Read(data)
	if err != nil {
		remote.window.PErr.Println("IPC server :: failed to receive data from : " + conn.RemoteAddr().String())
	}
	rxString := string(data[:size])
	if err = remote.window.Interpreter(rxString); err != nil {
		remote.window.PErr.Println("IPC server :: failed to interpret data from :" + conn.RemoteAddr().String())
	}
}

//ipc 커넥터
func (remote *remoteprocedure) Ipc_connector(android net.Conn) error {
	conn, err := net.Dial("tcp", "localhost:"+RPCLISTENINGPORT)
	if err != nil {
		remote.window.PErr.Println(color.RedString("IPC client :: failed to connect to python ipc server"))
		return err
	} else {
		//TODO: 보낼 데이터를 정의
		if _, err := conn.Write([]byte("write data here!!")); err != nil {
			return err
		}
		remote.window.PInfo.Println("executed command : INFO")
	}
	return nil
}

//프로시저 리스너
func RPC_Start(window *Window) error {
	resolver, err := net.ResolveTCPAddr("tcp", "0.0.0.0:"+RPCLISTENINGPORT)
	if err != nil {
		return err
	}

	receiver, err := net.ListenTCP("tcp", resolver)
	if err != nil {
		return err
	}

	remote := new(remoteprocedure)
	remote.window = window
	if err = rpc.Register(remote); err != nil {
		return err
	}

	for {
		rpc.Accept(receiver)
	}
}

//원격 프로시저
func (rc *remoteprocedure) RPC_TOAPP(data []byte, ack bool) error {
	if err := json.Unmarshal(data, *rc.window); err != nil {
		ack = false
		return err
	} else {
		ack = true
	}
	return nil
}
