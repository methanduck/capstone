package InteractiveSocket

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
)

const (
	//수신 명령옵션
	OPERATION_OPEN        = "OPEN"
	OPERATION_CLOSE       = "CLOSE"
	OPERATION_MODEAUTO    = "AUTO"
	OPERATION_INFORMATION = "INFO"
	OPERATION_PROXY       = "REMOTE"
	ERR_SELECTION         = "ERRSELECT"
	COMM_ERR              = "ERR"  // 유효하지 않은 명령이거나 서비스되고 있는 항목이 아닐 경우 ex) 존재하지 않는 동작 명령어 수신 시
	COMM_FAIL             = "FAIL" // 유효한 명령이었으나 정상적으로 처리되지 않은 경우 ex) 로그인 실패
	COMM_SUCCESS          = "OK"   // Transaction 성공 시 전송
	DELIMITER             = ","    //TODO : 정리가능
	FILENAME              = "WindowDATA.txt"
	MODE_PASSWDCONFIG     = "PASSWDCONFIG"
	MODE_VALIDATION       = "VALIDATION"
	POS_PASSWORD          = 0
	POS_HOSTNAME          = 1
	STATE_OFFLINE         = "OFFLINE"
	STATE_ONLINE          = "ONLINE"
)

type Node struct {
	Which        bool        `json:"which"` //창문 또는 어플리케이션을 구분 // 창문 : false , 어플리케이션 : true
	Initialized  bool        `json:"Initialized"`
	PassWord     string      //`json:"PassWord"` //창문 비밀번호
	IPAddr       string      `json:"IPAddr"`       // Window Address
	Identity     string      `json:"Identity"`     //중복되는 IP에서도 창문을 구별 할 수 있음 , md5로 해싱되어 저장됌
	ModeAuto     bool        `json:"ModeAuto"`     //자동 모드 설정
	ModeProxy    bool        `json:"mode_proxy"`   //중계서버 연결 설정
	Oper         string      `json:"Oper"`         // "OPEN", "CLOSE", "CONF", "INFO"  // 창문 : "INFO", "ONLINE"
	Ack          interface{} `json:"Ack"`          // "OK", "COMM_SUCCESS", "TRUE", "FAIL", "FALSE", "OFFLINE"
	Temp_IN      int         `json:"temp_in"`      // temperature - inside
	Temp_OUT     int         `json:"temp_out"`     // Temperature - outside
	Humidity_IN  int         `json:"humidity_in"`  // Humidity - inside
	Humidity_OUT int         `json:"humidity_out"` // Humidity	- outside
	Gas          int         `json:"Gas"`          // Gas
	Smoke        bool        `json:"smoke"`        // Smoke
	Light        int         `json:"Light"`        // Light
	Rain         bool        `json:"rain"`         // Rain
	Motion       bool        `json:"motion"`       // Motion
	Dust         int         `json:"dust"`         // Dust
}

func (node *Node) Authentication(input *Node) error {
	if node.PassWord != input.PassWord {
		return fmt.Errorf(COMM_ERR)
	}
	return nil
}

//자격증명
func (node *Node) HashValidation(passwd string, operation string) error {
	hashfunc := md5.New()
	switch operation {
	//Passwd set
	case MODE_PASSWDCONFIG:
		hashfunc.Write([]byte(passwd))
		node.PassWord = hex.EncodeToString(hashfunc.Sum(nil))
	//Passwd validation
	case MODE_VALIDATION:
		hashfunc.Write([]byte(passwd))
		if node.PassWord != hex.EncodeToString(hashfunc.Sum(nil)) {
			return errors.New("Validation fail")
		}
	}
	return nil
}

//Identity 설정
func (node *Node) SetIdentity() {
	hashfunc := md5.New()
	addr := node.GetMacAddr()
	hashfunc.Write([]byte(addr))
	node.Identity = hex.EncodeToString(hashfunc.Sum(nil))
	if node.Identity == "" {
		panic("Failed to set identity")
	}
}

//파일에 존재하는 설정값을 load함
func (node *Node) FILE_LOAD() error {
	fileInfo, err := os.Stat(FILENAME)
	if err != nil {
		return errors.New("File not found")
	}
	if file, err := os.OpenFile(FILENAME, os.O_RDWR|os.O_TRUNC, os.FileMode(0644)); err != nil {
		return errors.New("File load fail")
	} else {
		data := make([]byte, fileInfo.Size())
		if _, err := file.Read(data); err != nil {
			return errors.New("File read fail")
		}
		if fileInfo.Size() == 0 {
			return errors.New("File size is 0")
		}
		var tmpNode Node
		_ = json.Unmarshal(data, tmpNode)
		node.DATA_INITIALIZER(tmpNode, true)
	}
	return nil
}

//현재 설정값을 파일로 flushing함
func (node *Node) FILE_FLUSH() error {
	byted, _ := json.Marshal(node)
	if file, err := os.OpenFile(FILENAME, os.O_RDWR|os.O_TRUNC, os.FileMode(0644)); err != nil {
		return errors.New("File write fail")
	} else {
		defer func() {
			_ = file.Close()
		}()
		if _, err := file.Write(byted); err != nil {
			return errors.New("File write fail")
		}
	}
	return nil
}

//파일형태의 설정값이 없으면 새로 생성하고 있으면 불러옴
func (node *Node) FILE_INITIALIZE() error {
	if _, err := os.Stat(FILENAME); err != nil {
		file, err := os.OpenFile(FILENAME, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
		if err != nil {
			return errors.New("File open fail")
		}
		defer func() {
			_ = file.Close()
		}()
		node.DATA_INITIALIZER(Node{}, false)
	} else {
		node.SetIdentity()
		if err := node.FILE_LOAD(); err != nil {
			return err
		}
	}
	return nil
}

/*
//파일 확인 및 생성
func (node *Node) FILE_INITIALIZE() (bool, error) {
	if _, infoerr := os.Stat(FILENAME); infoerr != nil {
		//로컬 파일이 존재하지 않을 경우
		dataFile, err := os.OpenFile(FILENAME, os.O_CREATE|os.O_TRUNC|os.O_RDONLY, os.FileMode(0644))
		if err != nil {
			return false, err
		}
		defer dataFile.Close()
		fmt.Println("SocketSVR file open succeeded data initialize require!!")
	} else {
		//로컬 파일이 존재 할 경우
		fmt.Println("SocketSVR Found local datafile, commencing data initialize")
		content, _ := node.FILE_READ()
		if content == "" {
			return false, fmt.Errorf("SocketSVR found empty data file, need configuration")
		}
		splitedContent := strings.Split(content, ";")
		if len(splitedContent) == 1 {
			node.PassWord = string(splitedContent[POS_PASSWORD])
		} else {
			node.PassWord = string(splitedContent[POS_PASSWORD])
			node.Identity = string(splitedContent[POS_HOSTNAME])
		}
		fmt.Println("SocketSVR Data initialize completed")
		return true, nil
	}
	return true, nil

}
/*  ****deprecated****
//파일 출력
func (node *Node) FILE_WRITE(data string) error {
	dataFile, err := os.OpenFile(FILENAME, os.O_RDWR|os.O_TRUNC, os.FileMode(0644))
	defer dataFile.Close()
	if _, err = dataFile.Write([]byte(data)); err != nil {
		return err
	}
	return nil
}

//파일 입력
func (node *Node) FILE_READ() (string, error) {
	dataFile, err := os.OpenFile(FILENAME, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return "", err
	}
	defer dataFile.Close()

	fileInfo, _ := os.Stat(FILENAME)
	content := make([]byte, fileInfo.Size())
	dataFile.Read(content)
	return string(content), nil
}

//설정된 값들을 파일로 출력
func (node *Node) FILE_FLUSH() error {
	err := node.FILE_WRITE(node.PassWord + ";" + node.Identity)
	return err
}
*/

//입력된 구조체의 값을 적용
//mode false; 모든 자료 초기화 true ; 자동화 설정 위함
func (node *Node) DATA_INITIALIZER(inputData Node, mode bool) {
	if mode {
		//들어온 구조체의 값을 통해 초기화
		node.Initialized = true
		_ = node.HashValidation(inputData.PassWord, MODE_PASSWDCONFIG)
		node.ModeAuto = inputData.ModeAuto
	} else {
		//기본 초기화
		node.Initialized = false
		node.PassWord = "0000"
		node.SetIdentity()
		node.ModeAuto = false
		node.Oper = ""
		node.Temp_IN = 0
		node.Temp_OUT = 0
		node.Humidity_OUT = 0
		node.Humidity_IN = 0
		node.Gas = 0
		node.Light = 0
	}

}

//Mac addr 수집
func (node *Node) GetMacAddr() string {
	interfaces, err := net.Interfaces()
	var addr string
	if err == nil {
		for _, i := range interfaces {
			if i.Flags&net.FlagUp != 0 && bytes.Compare(i.HardwareAddr, nil) != 0 {
				addr = i.HardwareAddr.String()
				break
			}
		}
	}
	return addr
}

func (node *Node) PrintData() {
	tmp, _ := json.Marshal(node)
	fmt.Println(string(tmp))
}
