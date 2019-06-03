package RelaySVR

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/fatih/color"
	"github.com/methanduck/capstone/InteractiveSocket"
	"github.com/pkg/errors"
	"log"
	"time"
)

const (
	ONLINE        = true
	OFFLINE       = false
	NOTAVAILABLE  = "N/A"
	ERRPROCESSING = "PSERR"
	BUCKET_NODE   = "NODE"
	//update options
	UPDATE_ONLIE      = 0
	UPDATE_APPREQCONN = 1
	UPDATE_WINREQCONN = 2
	UPDATE_ALL        = 3
)

type dbData struct {
	database *bolt.DB
	pInfo    *log.Logger
	PErr     *log.Logger
}

type NodeState struct {
	Identity         string
	lastLogin        time.Time //TODO : 온라인 여부를 타임스탬프 계산으로 확인 할 수 있도록 전환 필요
	IsOnline         bool
	IsWinRequireconn bool
	IsAppRequireconn bool
	ApplicationData  InteractiveSocket.Node
	Locking          int //TODO : embedded locking 전환 필요
}

//기본적으로 온라인인 상태로 리턴
func NodeStateMaker(data InteractiveSocket.Node, state NodeState) NodeState {
	return NodeState{ApplicationData: data, Identity: data.Identity, IsOnline: ONLINE, IsWinRequireconn: state.IsWinRequireconn, Locking: 0}
}

//Starting BoltDB
//서버초기화에 실패 시 서버가 비정상 종료됩니다.
//Bucket : State, Node
func (db dbData) Startbolt(pinfo *log.Logger, perr *log.Logger) *bolt.DB {
	db.pInfo = pinfo
	db.PErr = perr
	boltdb, err := bolt.Open("SmartWindow.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Panic("Failed to open bolt database")
	}
	db.database = boltdb
	defer func() {
		if err := boltdb.Close(); err != nil {
			perr.Panic("bolt database abnormally terminated")
		}
	}()

	pinfo.Println(color.BlueString("BOLT : create new bucket"))
	//Node state bucket creation
	if err := boltdb.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("Node")); err != nil {
			return fmt.Errorf("bucket creation failed")
		}
		pinfo.Println(color.GreenString("BOLT : [OK] bucket creation secceded"))
		return nil
	}); err != nil {
		db.PErr.Panic(color.RedString("BOLT : Failed to  Start BoltDB (ERR code :" + err.Error() + ")"))
	}
	db.pInfo.Println(color.GreenString("BOLT : [OK] successfully database initialized"))
	return boltdb
}

//노드가 존재하는지 확인하고 존재한다면, 온라인 상태를 반환합니다.
//error : 데이터가 존재하지 않습니다, 데이터 처리중 오류가 발생했습니다.
//bool : 온라인이든 아니든 리턴이 이뤄질 경우 등록은 되어있는것으로 판단 가능
func (db dbData) IsExistAndIsOnline(identity string) (bool, error) {
	var isOnline bool
	tempNodeState := NodeState{}
	if err := db.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		val := bucket.Get([]byte(identity))
		if len(val) == 0 {
			return fmt.Errorf(NOTAVAILABLE)
		}
		if err := json.Unmarshal(val, tempNodeState); err != nil {
			return fmt.Errorf("BOLT : failed to unmarshal")
		}
		isOnline = tempNodeState.IsOnline
		return nil
	}); err != nil {
		if err.Error() == NOTAVAILABLE {
			return false, errors.New(NOTAVAILABLE)
		}
		return false, errors.New(ERRPROCESSING)
	} else {
		return isOnline, nil
	}
}

//해당하는 창문에 대한 명령 요청이 존재하는지 확인합니다.
// mode는 창문 또는 어플리케이션이 전달할 내용이 존재하는지 표현합니다.
//명령의 요청 여부는 res (bool)
//해당하는 창문이 데이터베이스에 존재하지 않으면 error를 반환합니다.
func (db dbData) IsRequireConn(identity string, mode string) (res bool, resErr error) {
	if err := db.database.View(func(tx *bolt.Tx) error {
		var tmp_NodeState NodeState
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		val := bucket.Get([]byte(identity))
		if len(val) == 0 {
			res = false
			resErr = errors.New("BOLT : Data not found")
		}
		_ = json.Unmarshal(val, tmp_NodeState) //에러 처리 안함
		switch mode {
		case "toapplication":
			res = tmp_NodeState.IsWinRequireconn
		case "towindow":
			res = tmp_NodeState.IsAppRequireconn
		}
		resErr = nil
		return nil
	}); err != nil {
		return
	}
	return
}

//Getting node data
func (db dbData) GetNodeData(identity string) (node NodeState, err error) {
	if err := db.database.View(func(tx *bolt.Tx) error {
		tmpNodeState := NodeState{}
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		val := bucket.Get([]byte(identity))
		if len(val) == 0 {
			node = NodeState{}
			err = errors.New("BOLT : DATA N/A")
		}
		_ = json.Unmarshal(val, &tmpNodeState) //에러 처리 안함
		node = tmpNodeState
		return nil
	}); err != nil {
		db.PErr.Println("BOLT : getting node data failed")
	}
	return
}

func (db dbData) PutNodeData(key string, val NodeState) error {
	err := db.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		marshaledVal, _ := json.Marshal(val)
		if err := bucket.Put([]byte(key), marshaledVal); err != nil {
			return errors.New("BOLT : Failed to put data")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

//Update the Node "State" as Online
func (db dbData) UpdataOnline(data InteractiveSocket.Node) error {
	var val []byte
	_, err := db.IsExistAndIsOnline(data.Identity)
	if err != nil { //err리턴 시 데이터가 존재하지 않거나 marshal 에러 발생
		temp_NodeState := NodeState{}
		if err := db.database.Update(func(tx *bolt.Tx) error {
			temp_NodeState.ApplicationData = data
			temp_NodeState.IsOnline = true
			temp_NodeState.Identity = data.Identity

			bucket := tx.Bucket([]byte(BUCKET_NODE))
			val, _ = json.Marshal(&temp_NodeState)
			if err := bucket.Put([]byte(data.Identity), val); err != nil {
				return errors.New("BOLT : Failed to put data")
			}
			return nil
		}); err != nil {
			return err
		}
	} else { //리턴이 존재하므로 데이터는 존재함
		temp_NodeState := NodeState{}
		if err := db.database.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(BUCKET_NODE))
			val = bucket.Get([]byte(data.Identity))
			if err := json.Unmarshal(val, &temp_NodeState); err != nil {
				return fmt.Errorf("BOLT : Failed to unmarshal")
			}
			//Modify state
			temp_NodeState.IsOnline = true
			temp_NodeState.lastLogin = time.Now()
			//marshalling
			val, err := json.Marshal(&temp_NodeState)
			if err != nil {
				return fmt.Errorf("BOLT : Failed to marshal")
			}
			//Update Node
			if err := bucket.Put([]byte(data.Identity), val); err != nil {
				fmt.Println(color.RedString("BOLT : Failed to update NodeState"))
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

//Update the Node data
func (db dbData) UpdateNodeDataState(data InteractiveSocket.Node, isOnline bool, isRequireConn bool, lock int, opts int) error {
	tmpNodeState := NodeState{}
	var val []byte
	if _, err := db.IsExistAndIsOnline(data.Identity); err != nil { //데이터가 존재하지 않을경우
		tmpNodeState.Identity = data.Identity
		tmpNodeState.IsOnline = isOnline
		tmpNodeState.IsWinRequireconn = isRequireConn
		tmpNodeState.Locking = lock
		if val, err := json.Marshal(&tmpNodeState); err != nil {
			return errors.New("BOLT : Failed to marshal")
		} else {
			if err := db.database.Update(func(tx *bolt.Tx) error {
				bucket := tx.Bucket([]byte(BUCKET_NODE))
				if err := bucket.Put([]byte(data.Identity), val); err != nil {
					return errors.New("BOLT : Failed to put data")
				}
				return nil
			}); err != nil {
				return err
			}
		}
	} else { //데이터가 존재할 경우
		tmp_NodeState := NodeState{}
		if err := db.database.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(BUCKET_NODE))
			val = bucket.Get([]byte(data.Identity))
			_ = json.Unmarshal(val, &tmp_NodeState) //에러 처리 하지 않음
			if tmp_NodeState.Locking == 1 {
				return errors.New("BOLT : Resource N/A")
			}
			switch opts {
			case UPDATE_ALL:
				tmp_NodeState.ApplicationData = data
				tmp_NodeState.IsOnline = isOnline
				tmp_NodeState.IsWinRequireconn = isRequireConn
				tmp_NodeState.Locking = lock
			case UPDATE_ONLIE:
				tmp_NodeState.IsOnline = isOnline
			case UPDATE_APPREQCONN:
				tmp_NodeState.IsAppRequireconn = isRequireConn
				if isRequireConn {
					tmpNodeState.ApplicationData = data
				}
			case UPDATE_WINREQCONN:
				tmpNodeState.IsWinRequireconn = isRequireConn
				if isRequireConn {
					tmpNodeState.ApplicationData = data
				}
			}

			val, _ = json.Marshal(&tmp_NodeState) //에러 처리 하지 않음
			if err := bucket.Put([]byte(data.Identity), val); err != nil {
				return errors.New("BOLT : faile to update data")
			}
			db.pInfo.Println("BOLT : successfully update data (window :" + data.Identity + "command :" + data.Oper + ")")
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

//Reset default state //default isOnline = false , isReqConn = false
func (db dbData) ResetState(identity string, isonline bool, isreqConn bool, lock int) error {
	tmpNodeState, err := db.GetNodeData(identity)
	if err != nil {
		return err
	}

	if isonline {
		tmpNodeState.IsOnline = true
	} else {
		tmpNodeState.IsOnline = false
	}
	if isreqConn {
		tmpNodeState.IsWinRequireconn = true
	} else {
		tmpNodeState.IsWinRequireconn = false
	}
	if lock == 1 {
		tmpNodeState.Locking = 1
	} else {
		tmpNodeState.Locking = 0
	}
	tmpNodeState.ApplicationData = InteractiveSocket.Node{}
	//put resetted struct
	if err := db.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		if val, err := json.Marshal(&tmpNodeState); err != nil {
			return errors.New("BOLT : Marshal failed")
		} else {
			if err := bucket.Put([]byte(identity), val); err != nil {
				return errors.New("BOLT : Put failed")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (db dbData) Walk() error {
	if err := db.database.View(func(tx *bolt.Tx) error {
		var unMarshalledVal NodeState
		bucket := tx.Bucket([]byte(BUCKET_NODE))
		cur := bucket.Cursor()
		_, val := cur.First()
		for val != nil {
			json.Unmarshal(val, unMarshalledVal)
			diff := time.Now().Sub(unMarshalledVal.lastLogin)
			if isActive := int(diff.Minutes()); isActive > 3 {
				unMarshalledVal.IsOnline = false
				if err := db.PutNodeData(unMarshalledVal.Identity, unMarshalledVal); err != nil {
					return err
				}
			}
			_, val = cur.Next()
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
