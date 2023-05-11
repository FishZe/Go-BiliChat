package handler

import (
	log "github.com/sirupsen/logrus"
	"reflect"
)

type HandlerFunc = func(event MsgEvent)

// FuncTable
// 空结构体不占空间，作为hashSet使用
// 函数指针内存唯一
type FuncTable = map[*HandlerFunc]struct{}

type RoomTable = map[int]FuncTable

type CmdTable = map[string]RoomTable

type Path struct {
	Cmd    string
	RoomId int
}

type Handler struct {
	CmdChan chan map[string]interface{}
	DoFunc  CmdTable
	//函数反查表
	FuncPath map[*HandlerFunc]Path
}

type jsonCoder interface {
	Unmarshal(data []byte, v interface{}) error
	Marshal(v interface{}) ([]byte, error)
}

var JsonCoder jsonCoder

func (handler *Handler) AddOption(Cmd string, RoomId int, Do HandlerFunc) *HandlerFunc {
	if _, ok := handler.DoFunc[Cmd]; !ok {
		handler.DoFunc[Cmd] = make(RoomTable)
	}
	if _, ok := handler.DoFunc[Cmd][RoomId]; !ok {
		handler.DoFunc[Cmd][RoomId] = make(FuncTable)
	}
	//将函数添加进Set
	handler.DoFunc[Cmd][RoomId][&Do] = struct{}{}
	//函数反查表
	handler.FuncPath[&Do] = Path{
		Cmd:    Cmd,
		RoomId: RoomId,
	}
	log.Debug("Add Option: ", Cmd, RoomId)
	return &Do
}

func (handler *Handler) DelRoomOption(roomId int) {
	// TODO: 检查这里是否需要用sync.Map
	for k, v := range handler.DoFunc {
		if _, ok := v[roomId]; ok {
			delete(handler.DoFunc[k], roomId)
			log.Debug("Del Option: ", k, roomId)
		}
	}
}

func (handler *Handler) DelOption(p *HandlerFunc) {
	if p != nil {
		path := handler.FuncPath[p]
		delete(handler.DoFunc[path.Cmd][path.RoomId], p)
		delete(handler.FuncPath, p)
	}
}

func (handler *Handler) doHandler(f reflect.Value, msg map[string]interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("doHandler panic: %v, please make an issue to me!", err)
		}
	}()
	res := f.Call([]reflect.Value{reflect.ValueOf(msg)})
	msgEvent := res[0].Interface().(MsgEvent)
	// 执行函数
	if msgEvent.Cmd != "" && msgEvent.RoomId != 0 {
		if cmd, ok := msg["cmd"].(string); ok {
			for t := range handler.DoFunc[cmd][msgEvent.RoomId] {
				log.Debugf("distribute %v cmd", msg["cmd"].(string))
				go (*t)(msgEvent)
			}
		}
	}
}

func (handler *Handler) CmdHandler() {
	for {
		select {
		case msg, ok := <-handler.CmdChan:
			if ok {
				//命令是字符串
				if cmd, ok := msg["cmd"].(string); ok {
					// 处理命令存在
					if dict, ok := handler.DoFunc[cmd]; ok {
						// 处理房间存在
						_, ok1 := dict[msg["RoomId"].(int)]
						// 0 为所以房间
						_, ok2 := dict[0]
						if ok1 || ok2 {
							setFunc := reflect.ValueOf(&Handler{}).MethodByName("Set" + CmdName[cmd])
							if setFunc.IsValid() {
								handler.doHandler(setFunc, msg)
							} else {
								log.Debug(CmdName[cmd] + " not found, please make an issue to me!")
							}
						}
					}
				}
			}
		}
	}
}
