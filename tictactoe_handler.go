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

func (t *TicTacToeHandler) OnInit(room gsi.Room) {
	if timeStr, ok := room.Metadata()["time_per_turn"]; ok {
		if timePerMove, err := strconv.ParseInt(timeStr, 10, 64); err != nil {
			t.timePerMove = timePerMove
		}
	}
}

func (t *TicTacToeHandler) AllowJoin(room gsi.Room, user *gsi.User) *gsi.CheckJoinConditionResult {
	return &gsi.CheckJoinConditionResult{
		Allow:  true,
		Reason: "",
	}
}

func (t *TicTacToeHandler) Processor(room gsi.Room, action string, data map[string]interface{}) {
	switch action {
	case "end game":
		t.endGame(room, data["winner"].(string))
	}
}


func (t *TicTacToeHandler) OnJoined(room gsi.Room, user *gsi.User) {
	player := &TicTacToePlayer{
		sid:         user.SID,
		id:          user.ID,
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
		if err := room.GetScheduler().Schedule("end game", map[string]interface{}{
			"winner": t.GetOtherID(user.ID).String(),
		}, time.Duration(t.timePerMove) *time.Second); err != nil {
			room.Logger().Error("schedule error", zap.Error(err))
		}
	}
}

func (t *TicTacToeHandler) OnReceived(room gsi.Room, user *gsi.User, message []byte) {
	if t.board.end() {
		room.SendError(user.SID, 0, "game has finished")
		return
	}

	var evt pb.TTT
	if err := proto.Unmarshal(message, &evt); err != nil {
		room.Logger().Error("unmarshal error", zap.Error(err))
		return
	}

	switch evt.Message.(type) {
	case *pb.TTT_Move:
		if t.turn != user.ID {
			room.SendError(user.SID, 0,  "not your turn")
			return
		} else {
			move := evt.GetMove()

			if end, err := t.board.doMove(move.Row, move.Col, move.Digit); err != nil {
				room.SendError(user.SID, 0, err.Error())
				return
			} else {
				room.GetScheduler().CancelIfExist("end game")
				t.sendAllRoomMessage(room, &pb.TTT{
					Message: &pb.TTT_PlayerMove{PlayerMove: &pb.PlayerMove{
						Id:    user.ID.String(),
						Row:   move.Row,
						Col:   move.Col,
						Digit: move.Digit,
					}},
				})

				if end {
					t.endGame(room, user.ID.String())
				} else {
					t.turn = t.GetOtherID(user.ID)
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
						if err := room.GetScheduler().Schedule("end game", map[string]interface{}{
							"winner": user.ID.String(),
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

func (t *TicTacToeHandler) OnLeft(room gsi.Room, user *gsi.User) {
	t.endGame(room, t.GetOtherID(user.ID).String())
}

func (t *TicTacToeHandler) endGame(room gsi.Room, winner string) {
	room.GetScheduler().CancelAll()
	t.sendAllRoomMessage(room, &pb.TTT{
		Message: &pb.TTT_End{End: &pb.End{
			Winner: winner,
		}},
	})
	room.Destroy()
}

func (t *TicTacToeHandler) OnClose(room gsi.Room) {
	room.GetScheduler().Stop()
	room.GetPusher().Stop()
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

func (t *TicTacToeHandler) sendAllRoomMessage(room gsi.Room, evt *pb.TTT) {
	if len(room.Players()) == 0 {
		return
	}

	if data, err := proto.Marshal(evt); err == nil {
		room.SendAll(data)
	} else {
		room.Logger().Error(err.Error())
	}

}

func (t *TicTacToeHandler) sendRoomMessage(room gsi.Room, id uuid.UUID, evt *pb.TTT) {
	if data, err := proto.Marshal(evt); err == nil {
		room.Send(id, data)
	} else {
		room.Logger().Error(err.Error())
	}
}
