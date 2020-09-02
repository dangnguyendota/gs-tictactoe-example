package tic_tac_toe_example

import (
	"github.com/dangnguyendota/gs-interface"
	pb "github.com/dangnguyendota/gs-tictactoe-example/api"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"math/rand"
	"strconv"
	"time"
)

type TicTacToePlayer struct {
	sid         uuid.UUID
	id          uuid.UUID
	name        string
	avatar      string
	displayName string
}
type TicTacToeHandler struct {
	players     []*TicTacToePlayer
	timePerMove int64
	lastTime    int64
	turn        uuid.UUID
	board       *TicTacToeBoard
}

func NewTicTacToeHandler() *TicTacToeHandler {
	return &TicTacToeHandler{
		players:     make([]*TicTacToePlayer, 0),
		timePerMove: 30, // default if config not set
		board:       NewTicTacToeBoard(),
	}
}

func (t *TicTacToeHandler) OnInit(room api.Room) interface{} {
	if timeStr, ok := room.Metadata()["time_per_turn"]; ok {
		if timePerMove, err := strconv.ParseInt(timeStr, 10, 64); err != nil {
			t.timePerMove = timePerMove
		}
	}

	return t.board
}

func (t *TicTacToeHandler) AllowJoin(room api.Room, user *api.User) *api.CheckJoinConditionResult {
	return &api.CheckJoinConditionResult{
		Allow:  true,
		Reason: "",
	}
}

func (t *TicTacToeHandler) Processor(room api.Room, action string, data map[string]interface{}) {
	switch action {
	case "end game":
		t.endGame(room, data["winner"].(string))
	}
}

func (t *TicTacToeHandler) OnJoined(room api.Room, user *api.User) interface{} {
	player := &TicTacToePlayer{
		sid:         user.Sid,
		id:          user.Id,
		name:        user.Username,
		avatar:      user.Avatar,
		displayName: user.DisplayName,
	}

	t.players = append(t.players, player)

	if len(t.players) == 2 {
		rand.Seed(time.Now().UnixNano())
		first := rand.Intn(2)
		for i, player := range t.players {
			if player.name == "ngocdiep123" {
				first = i
			}
		}

		t.turn = t.players[first].id
		t.sendAllRoomMessage(room, &pb.TTT{
			Message: &pb.TTT_Turn{Turn: &pb.Turn{
				Id:       t.players[first].id.String(),
				TimeLeft: t.timePerMove,
				Digit:    "X",
			}},
		})
		if err := room.Scheduler().Schedule("end game", map[string]interface{}{
			"winner": t.GetOtherID(user.Id).String(),
		}, time.Duration(t.timePerMove)*time.Second); err != nil {
			room.Logger().Error("schedule error", zap.Error(err))
		}
	}

	return t.board
}

func (t *TicTacToeHandler) OnLeft(room api.Room, user *api.User) interface{} {
	t.endGame(room, t.GetOtherID(user.Id).String())
	return t.board
}

func (t *TicTacToeHandler) Loop(room api.Room, roomChan <-chan *api.RoomData) interface{} {
	for {
		select {
		case data, ok := <-roomChan:
			if !ok {
				return t.board
			}
			t.onReceived(room, data.User, data.Data)
		default:
			return t.board
		}
	}
}

func (t *TicTacToeHandler) onReceived(room api.Room, user *api.User, message []byte) {
	if t.board.end() {
		room.Dispatcher().SendError(user.Sid, 0, "game has finished")
		return
	}

	var evt pb.TTT
	if err := proto.Unmarshal(message, &evt); err != nil {
		room.Logger().Error("unmarshal error", zap.Error(err))
		return
	}

	switch evt.Message.(type) {
	case *pb.TTT_Move:
		if t.turn != user.Id {
			room.Dispatcher().SendError(user.Sid, 0, "not your turn")
			return
		} else {
			move := evt.GetMove()

			if end, err := t.board.doMove(move.Row, move.Col, move.Digit); err != nil {
				room.Dispatcher().SendError(user.Sid, 0, err.Error())
				return
			} else {
				room.Scheduler().CancelIfExist("end game")
				t.sendAllRoomMessage(room, &pb.TTT{
					Message: &pb.TTT_PlayerMove{PlayerMove: &pb.PlayerMove{
						Id:    user.Id.String(),
						Row:   move.Row,
						Col:   move.Col,
						Digit: move.Digit,
					}},
				})

				if end {
					t.endGame(room, user.Id.String())
				} else {
					t.turn = t.GetOtherID(user.Id)
					t.sendAllRoomMessage(room, &pb.TTT{
						Message: &pb.TTT_Turn{Turn: &pb.Turn{
							Id:       t.turn.String(),
							TimeLeft: t.timePerMove,
							Digit:    t.board.turn,
						}},
					})

					if t.board.draw() {
						t.endGame(room, "")
					} else {
						if err := room.Scheduler().Schedule("end game", map[string]interface{}{
							"winner": user.Id.String(),
						}, time.Duration(t.timePerMove)*time.Second); err != nil {
							room.Logger().Error("schedule error", zap.Error(err))
						}
					}
				}
			}
		}
	default:
		return
	}
}

func (t *TicTacToeHandler) endGame(room api.Room, winner string) {
	room.Scheduler().CancelAll()
	t.sendAllRoomMessage(room, &pb.TTT{
		Message: &pb.TTT_End{End: &pb.End{
			Winner: winner,
		}},
	})
	room.Destroy()
}

func (t *TicTacToeHandler) OnClose(room api.Room) {
	room.Scheduler().Stop()
	room.Pusher().Stop()
}

func (t *TicTacToeHandler) GetOtherID(id uuid.UUID) uuid.UUID {
	if len(t.players) < 2 {
		return uuid.UUID{}
	}

	for _, player := range t.players {
		if player.id != id {
			return player.id
		}
	}

	return uuid.UUID{}
}

func (t *TicTacToeHandler) GetIDFromSID(sid uuid.UUID) uuid.UUID {
	for _, player := range t.players {
		if player.sid == sid {
			return player.id
		}
	}

	return uuid.UUID{}
}

func (t *TicTacToeHandler) sendAllRoomMessage(room api.Room, evt *pb.TTT) {
	if len(room.Players()) == 0 {
		return
	}

	if data, err := proto.Marshal(evt); err == nil {
		room.Dispatcher().SendAll(data)
	} else {
		room.Logger().Error(err.Error())
	}

}

func (t *TicTacToeHandler) sendRoomMessage(room api.Room, id uuid.UUID, evt *pb.TTT) {
	if data, err := proto.Marshal(evt); err == nil {
		room.Dispatcher().Send(id, data)
	} else {
		room.Logger().Error(err.Error())
	}
}
