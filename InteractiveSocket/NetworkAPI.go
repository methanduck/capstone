package InteractiveSocket

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//고정 변수
const (
	RPCLISTENINGPORT = "66866"
	//중계서버 IP
	RELAYSVRIPADDR = "127.0.0.1:6866"
)

type Window struct {
	PInfo          *log.Logger
	PErr           *log.Logger
	svrInfo        *Node
	Available      *sync.Mutex
	FAvailable     *sync.Mutex
	quitSIGNAL     chan string
	python         *python
	ipc            *remoteprocedure
	completeSIGNAL chan bool
}
type python struct {
	path         string
	filename     string
	pythonclient string
	pythonifo    string
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
		win.svrInfo.Oper = OPERATION_INFORMATION
		win.Operation(*win.svrInfo, nil)
		//win.COMM_ACK("Configuration require", Android)
		_ = COMM_SENDJSON(*win.svrInfo, Android)
		recvData, err := COMM_RECVJSON(Android)
		if err != nil {
			win.PErr.Println(color.RedString("Failed to receive json"))
		} else {
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
				_ = COMM_SENDJSON(*win.svrInfo, conn)
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
	defer win.Available.Unlock()

	conn, err := net.Dial("tcp", "127.0.0.1:"+win.python.pythonclient)

	switch order.Oper {
	case OPERATION_OPEN:
		if err != nil {
			win.PErr.Println(color.RedString("failed to run command : OPEN (err code : " + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			if _, err := conn.Write([]byte("OPEN")); err != nil {
				win.PErr.Println(color.RedString("IPC client :: failed to send command : OPEN"))
				win.COMM_ACK(COMM_FAIL, android)
			}
			win.COMM_ACK(COMM_SUCCESS, android)
			if err := conn.Close(); err != nil {
				win.PErr.Println(color.RedString("IPC client :: connection terminated abnormaly"))
			}
			win.PInfo.Println(color.GreenString("executed command : OPEN"))
		}

	case OPERATION_CLOSE:
		if err != nil {
			win.PErr.Println(color.RedString("failed to run command : CLOSE (err code :" + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			if _, err := conn.Write([]byte("CLOSE")); err != nil {
				win.PErr.Println(color.RedString("IPC client :: failed to send command : CLOSE"))
				win.COMM_ACK(COMM_FAIL, android)
			}
			win.COMM_ACK(COMM_SUCCESS, android)

			if err := conn.Close(); err != nil {
				win.PErr.Println(color.RedString("IPC client :: connection terminated abnormaly"))
			}
			win.PInfo.Println(color.GreenString("executed command : CLOSE"))
		}

	case OPERATION_INFORMATION:
		if err != nil {
			win.PErr.Println(color.RedString("failed to run command : INFO (err code :" + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			data := make([]byte, 500)
			size, err := conn.Read(data)
			if err != nil {
				win.PErr.Println(color.RedString("IPC client :: failed to receive information : INFO"))
				win.COMM_ACK(COMM_FAIL, android)
			} else {
				if err := win.Interpreter(string(data[:size])); err != nil {
					win.PErr.Println(color.RedString(err.Error()))
				} else {
					if android != nil {
						_ = COMM_SENDJSON(win.svrInfo, android)
					}
					win.PInfo.Println(color.GreenString("executed command : INFO"))
				}
			}
		}

	case OPERATION_MODEAUTO:
		if err != nil {
			win.PErr.Println(color.RedString("failed to run command : MODEAUTO (err code :" + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			if order.ModeAuto {
				if _, err := conn.Write([]byte("AUTO 1 " + strconv.Itoa(order.Humidity_IN) + " " + strconv.Itoa(order.Temp_IN))); err != nil {
					win.PErr.Println(color.RedString("IPC client :: failed to send command : MODEAUTO"))
					win.COMM_ACK(COMM_FAIL, android)
				} else {
					win.COMM_ACK(COMM_SUCCESS, android)
				}
			} else {
				if _, err := conn.Write([]byte("AUTO 0")); err != nil {
					win.PErr.Println(color.RedString("IPC client :: failed to send command : MODEAUTO"))
					win.COMM_ACK(COMM_FAIL, android)
				} else {
					win.PInfo.Println(color.GreenString("executed command : MODEAUTO"))
					win.COMM_ACK(COMM_SUCCESS, android)
				}
			}
		}
		//온도, 습도, 조도, 인체감지

	case OPERATION_PROXY:
		win.svrInfo.ModeProxy = order.ModeProxy
		if win.svrInfo.ModeProxy {
			win.updateToRelaySVR()
		} else {
			win.close_UpdateToRerlaySVR()
		}
		win.COMM_ACK(COMM_SUCCESS, android)

	case OPERATION_CLEAR:
		if err != nil {
			win.PErr.Println(color.RedString("failed to run command : CLEAR (err code :" + err.Error() + ")"))
			win.COMM_ACK(COMM_FAIL, android)
		} else {
			if _, err := conn.Write([]byte("CLEAR")); err != nil {
				win.PErr.Println(color.RedString("IPC client :: failed to send command : CLEAR"))
				win.COMM_ACK(COMM_FAIL, android)
			} else {
				win.PInfo.Println(color.GreenString("executed command : CLEAR"))
				win.COMM_ACK(COMM_SUCCESS, android)
			}
		}

	default:
		win.PErr.Println(color.RedString("received not compatible command (OPER)"))
		win.COMM_ACK(COMM_FAIL, android)
	}
}

//응답 송신
func (win *Window) COMM_ACK(result string, android net.Conn) {
	win.svrInfo.Ack = result
	res, _ := json.Marshal(win.svrInfo)
	_, _ = android.Write(res)
}

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

//프로그램 시작부
func (win *Window) Start(address string, port string, path string, filename string, pythonclient string, pythoninfo string) error {
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

	win.python.pythonclient = pythonclient
	win.python.pythonifo = pythoninfo
	if err := win.svrInfo.FILE_INITIALIZE(); err != nil {
		win.PErr.Println(err)
	} else {
		win.PInfo.Println(color.GreenString("[OK] File loaded"))
	}
	//서버 리스닝 시작부
	Android, err := net.Listen("tcp", address+":"+port)
	if err != nil {
		win.PErr.Fatal(color.RedString("[ERR] failed to open socket ( address :" + address + " port :" + port + "), (err code :" + err.Error() + ", Abort"))
		return err
	} else {
		win.PInfo.Println(color.GreenString("[OK] initialized = " + address + ":" + port))
		win.PInfo.Println("#############################Currently configured data################################")
		win.svrInfo.PrintData()
		win.PInfo.Println("######################################################################################")
		win.PInfo.Println(color.GreenString("[OK] configured parameter"))
		win.PInfo.Println("###########################Currently configured parameter#############################")
		win.PInfo.Println("Python path : " + win.python.path)
		win.PInfo.Println("Python file name : " + win.python.filename)
		win.PInfo.Println("######################################################################################")
	}

	defer func() {
		err := Android.Close()
		if err != nil {
			win.PErr.Println(color.RedString("terminated abnormaly" + Android.Addr().String()))
		}
	}()

	for {
		connect, err := Android.Accept()
		if err != nil {
			win.PErr.Println(color.RedString("failed to connect TCP with :" + connect.RemoteAddr().String()))
		} else {
			win.PInfo.Println("successfully TCP connected with :" + connect.RemoteAddr().String())
			//start go routine
			go win.afterConnected(connect)
		}
		defer func() {
			err := connect.Close()
			if err != nil {
				win.PErr.Println(color.RedString("connection terminated abnormaly with client :" + Android.Addr().String()))
			}
		}()
	}
}

//센서 데이터 해석
func (win *Window) Interpreter(data string) (err error) {
	result := strings.Split(data, DELIMITER)
	win.svrInfo.Gas, err = strconv.ParseBool(result[0])
	win.svrInfo.Rain, err = strconv.ParseBool(result[1])
	win.svrInfo.Light, err = strconv.Atoi(result[2])
	win.svrInfo.Motion, err = strconv.ParseBool(result[3])
	win.svrInfo.Humidity_IN, err = strconv.Atoi(result[4])
	win.svrInfo.Temp_IN, err = strconv.Atoi(result[5])
	win.svrInfo.Smoke, err = strconv.ParseBool(result[6])
	win.svrInfo.Dust, err = strconv.Atoi(result[7])
	win.svrInfo.Humidity_OUT, err = strconv.Atoi(result[8])
	win.svrInfo.Temp_OUT, err = strconv.Atoi(result[9])
	win.svrInfo.IsOPEN, err = strconv.ParseBool(result[10])
	win.svrInfo.IsFILM, err = strconv.ParseBool(result[11])
	return
}
