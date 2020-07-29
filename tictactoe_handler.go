package tic_tac_toe_example

import (
	"github.com/dangnguyendota/gs-interface"
	"github.com/dangnguyendota/gs-interface/gs_proto"
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
	gsi.RoomHandler
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

func (t *TicTacToeHandler) HandleJoin(room gsi.Room, user *gsi.User) {
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

func (t *TicTacToeHandler) HandleLeave(room gsi.Room, user *gsi.User) {
	t.endGame(room, t.GetOtherID(user.ID).String())
}

func (t *TicTacToeHandler) HandleData(room gsi.Room, message *gsi.RoomDataMessage) {
	if t.board.end() {
		room.Send(message.From, &ip.Packet{
			PacketId: "",
			Message: &ip.Packet_Error{Error: &ip.Error{
				Code:    0,
				Message: "game has finished",
			}},
		})
		return
	}

	from := t.GetIDFromSID(message.From)
	var evt pb.TTT
	if err := proto.Unmarshal(message.Data, &evt); err != nil {
		room.Logger().Error("unmarshal error", zap.Error(err))
		return
	}

	switch evt.Message.(type) {
	case *pb.TTT_Move:
		if t.turn != from {
			room.Send(message.From, &ip.Packet{
				PacketId: "",
				Message: &ip.Packet_Error{Error: &ip.Error{
					Code:    0,
					Message: "not your turn",
				}},
			})
			return
		} else {
			move := evt.GetMove()

			if end, err := t.board.doMove(move.Row, move.Col, move.Digit); err != nil {
				room.Send(message.From, &ip.Packet{
					PacketId: "",
					Message: &ip.Packet_Error{Error: &ip.Error{
						Code:    0,
						Message: err.Error(),
					}},
				})
				return
			} else {
				room.GetScheduler().CancelIfExist("end game")
				t.sendAllRoomMessage(room, &pb.TTT{
					Message: &pb.TTT_PlayerMove{PlayerMove: &pb.PlayerMove{
						Id:    from.String(),
						Row:   move.Row,
						Col:   move.Col,
						Digit: move.Digit,
					}},
				})

				if end {
					t.endGame(room, from.String())
				} else {
					t.turn = t.GetOtherID(from)
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
							"winner": from.String(),
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
		room.SendAll(&ip.Packet{
			Message: &ip.Packet_RoomMessage{
				RoomMessage: &ip.RoomMessage{
					RoomType: room.Game(),
					RoomId:   room.ID().String(),
					From:     "server",
					Data:     data,
					Code:     -1,
					Time:     time.Now().Unix(),
				},
			},
		})
	} else {
		room.Logger().Error(err.Error())
	}

}

func (t *TicTacToeHandler) sendRoomMessage(room gsi.Room, id uuid.UUID, evt *pb.TTT) {
	if data, err := proto.Marshal(evt); err == nil {
		room.Send(id, &ip.Packet{
			Message: &ip.Packet_RoomMessage{
				RoomMessage: &ip.RoomMessage{
					RoomType: room.Game(),
					RoomId:   room.ID().String(),
					From:     "server",
					Data:     data,
					Code:     -1,
					Time:     time.Now().Unix(),
				},
			},
		})
	} else {
		room.Logger().Error(err.Error())
	}
}
