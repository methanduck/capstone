/*
	Main Relay Server
*/
package RelaySVR

import (
	"encoding/json"
	"github.com/fatih/color"
	"github.com/methanduck/capstone/InteractiveSocket"
	"github.com/pkg/errors"
	"log"
	"net"
	"os"
	"time"
)

const (
	Service_port = "6866"
)

type Server struct {
	State    *dbData
	SVR_Addr string
	SVR_Port string
	Pinfo    *log.Logger
	PErr     *log.Logger
	// ctx		 context.Context TODO Context 추가 여부 검토
}

//Start Serer
func (server *Server) Start(address string, port string) error {
	server.Pinfo = log.New(os.Stdout, "INFO :", log.LstdFlags)
	server.PErr = log.New(os.Stdout, color.RedString("ERR :"), log.LstdFlags)

	//bolt database initializing
	server.State = new(dbData)
	go server.State.Startbolt(server.Pinfo, server.PErr)
	Listener, err := net.Listen("tcp", server.SVR_Addr+":"+server.SVR_Port)
	if err != nil {
		server.PErr.Panic("Failed to open server (Err code : %s ", err)
	} else {
		server.Pinfo.Println(color.BlueString("[OK] Relay server initiated " + address + ":" + port))
	}

	defer func() {
		if err := Listener.Close(); err != nil {
			server.PErr.Panic(color.RedString("Abnormal termination while closing server"))
		}
	}()

	//TODO: database trash cleaner
	go func() {

	}()

	for {
		if connection, err := Listener.Accept(); err != nil {
			server.PErr.Println("Failed to connect :" + connection.RemoteAddr().String())
		} else {
			//수신 시
			go func() {
				server.afterConnected(connection)
			}()
		}

	}
}

//통신이 수립되었을 때 수행하는 함수
func (server Server) afterConnected(conn net.Conn) {

	//Json 해석된 result struct
	result, err := InteractiveSocket.COMM_RECVJSON(conn)
	if err != nil {
		server.PErr.Println(err)
	}
	switch result.Which {
	//Application
	case true:
		status, err := server.State.IsExistAndIsOnline(result.Identity)
		if err != nil {
			server.Pinfo.Println("Send Ack : ERR")
			if err := server.ackResult(InteractiveSocket.COMM_FAIL, conn); err != nil {
				server.PErr.Println(color.RedString(err.Error()))
			}
		}
		if !status { //서버에서 offline일 경우 조종이 불가하여 offline응답을 전송
			if err := server.ackResult(InteractiveSocket.STATE_OFFLINE, conn); err != nil {
				server.PErr.Println(color.RedString(err.Error()))
			}
		} else { //online확인
			if err := server.State.UpdateNodeDataState(result, false, true, 1, UPDATE_APPREQCONN); err != nil {
				server.PErr.Println(err)
				_ = InteractiveSocket.COMM_SENDJSON(&InteractiveSocket.Node{Ack: err.Error()}, conn) //TODO : 오류 종류에 대한 처리 없이 오류 사항을 그대로 전송중
			}
			//어플리케이션 명령 처리
			switch result.Oper {
			case InteractiveSocket.OPERATION_INFORMATION:
				time.Sleep(3 * time.Second)
				if window, err := server.State.GetNodeData(result.Identity); err != nil {
					server.PErr.Println(err)
				} else {
					window.ApplicationData.Ack = InteractiveSocket.COMM_SUCCESS
					_ = InteractiveSocket.COMM_SENDJSON(&window.ApplicationData, conn)
					if err := server.State.ResetState(result.Identity, true, false, 0); err != nil {
						server.PErr.Println(err)
					}
				}
			case InteractiveSocket.OPERATION_OPEN, InteractiveSocket.OPERATION_CLOSE, InteractiveSocket.OPERATION_PROXY, InteractiveSocket.OPERATION_MODEAUTO:
				window, _ := server.State.GetNodeData(result.Identity)
				if window.Locking == 1 {
					if err := server.ackResult(InteractiveSocket.COMM_ERR, conn); err != nil {
						server.PErr.Println(color.RedString("failed to send ack message (err code : %s )", err.Error()))
					}
				} else {
					_ = server.State.UpdateNodeDataState(result, false, true, 1, UPDATE_APPREQCONN)
				}

			default:
				if err := server.ackResult(InteractiveSocket.COMM_ERR, conn); err != nil {
					server.PErr.Println(color.RedString("failed to send ack message (err code : %s )", err.Error()))
				}
				server.PErr.Println(color.RedString("Received N/A command"))
			}
		}
	//Window
	//창문의 경우 한번이라도 신호를 보내오면 온라인 연결 간주, 대기중인 명령이 있는지 확인 후 명령 처리 및 응답
	case false:
		isconn, err := server.State.IsRequireConn(result.Identity, "towindow")
		if err != nil {
			if err := server.ackResult(InteractiveSocket.COMM_FAIL, conn); err != nil {
				server.PErr.Println(color.RedString("failed to send ack message (err code : %s )", err.Error()))
			}
		}
		switch result.Oper {
		case InteractiveSocket.OPERATION_INFORMATION:
			if err := server.State.UpdateNodeDataState(result, true, false, 1, UPDATE_ALL); err != nil {
				server.PErr.Println(err)
			}

		case InteractiveSocket.STATE_ONLINE: //주기적 수신
			if err := server.State.UpdataOnline(result); err != nil {
				server.PErr.Println(err)
			} else {
				blue := color.New(color.BgBlue).SprintfFunc()
				server.Pinfo.Println(blue("Widow state updated (window :" + result.Identity + ", state : online"))
				if isconn {
					data, _ := server.State.GetNodeData(result.Identity)
					_ = InteractiveSocket.COMM_SENDJSON(&data.ApplicationData, conn)
					_ = server.State.UpdateNodeDataState(result, false, false, 0, UPDATE_APPREQCONN)
				} else {
					if err := server.ackResult(InteractiveSocket.COMM_SUCCESS, conn); err != nil {
						server.PErr.Println(color.RedString("failed to send ack message (err code : %s )", err.Error()))
					}
					_ = InteractiveSocket.COMM_SENDJSON(&InteractiveSocket.Node{Ack: InteractiveSocket.COMM_SUCCESS}, conn)
				}
			}

		//case InteractiveSocket.:

		default:
			server.PErr.Println("Received N/A command")
		}
	}
}

func (server Server) ackResult(ack string, conn net.Conn) error {
	result := make(map[string]string)
	result["Ack"] = ack

	data, _ := json.Marshal(result)
	if _, err := conn.Write(data); err != nil {
		return errors.New("failed to send ack message (err code :" + err.Error() + ")")
	} else {
		return nil
	}
}
