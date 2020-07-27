package tic_tac_toe_example

import (
	"errors"
)

type TicTacToeBoard struct {
	board     [][]string
	turn      string
	moveCount int
}

func NewTicTacToeBoard() *TicTacToeBoard {
	return &TicTacToeBoard{
		board: [][]string{
			{"", "", ""},
			{"", "", ""},
			{"", "", ""},
		},
		turn: "X",
		moveCount: 0,
	}
}

// end game?
func (t *TicTacToeBoard) doMove(row, col int32, digit string) (bool, error) {
	if t.turn != digit {
		return false, errors.New("not your turn")
	}

	if row < 0 || row > 2 || col < 0 || col > 2 {
		return false, errors.New("invalid square")
	}

	if t.board[row][col] != "" {
		return false, errors.New("square is not empty")
	}

	if digit != "X" && digit != "O" {
		return false, errors.New("invalid input")
	}

	t.board[row][col] = digit
	t.moveCount++
	if digit == "O" {
		t.turn = "X"
	} else {
		t.turn = "O"
	}

	return t.end(), nil
}

func (t *TicTacToeBoard) end() bool {
	if t.board[0][0] != "" && t.board[0][0] == t.board[0][1] && t.board[0][0] == t.board[0][2] {
		return true
	}
	if t.board[1][0] != "" && t.board[1][0] == t.board[1][1] && t.board[1][0] == t.board[1][2] {
		return true
	}
	if t.board[2][0] != "" && t.board[2][0] == t.board[2][1] && t.board[2][0] == t.board[2][2] {
		return true
	}

	if t.board[0][0] != "" && t.board[0][0] == t.board[1][0] && t.board[0][0] == t.board[2][0] {
		return true
	}

	if t.board[0][1] != "" && t.board[0][1] == t.board[1][1] && t.board[0][1] == t.board[2][1] {
		return true
	}

	if t.board[0][2] != "" && t.board[0][2] == t.board[1][2] && t.board[0][2] == t.board[2][2] {
		return true
	}

	if t.board[0][0] != "" && t.board[0][0] == t.board[1][1] && t.board[0][0] == t.board[2][2] {
		return true
	}

	if t.board[0][2] != "" && t.board[0][2] == t.board[1][1] && t.board[0][2] == t.board[2][0] {
		return true
	}

	return false
}

func (t *TicTacToeBoard) draw() bool {
	return t.moveCount == 9 && !t.end()
}
