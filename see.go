package dragontoothmg

import (
	"math/bits"
)

// This new addition aims to add functionality useful to adding a static exchange evaluation
// The main challenge is to handle cases where two pieces are aligned along a diagonal or file,
// Thus they can take sequentially and thus dont interpose
// The only edge case I haven't managed to cover is where a bishop or queen stares through a pawn,
// effectively adding an attacker to the pawn's attacking square (just hard to do with bitboards)

func (b *Board) GenerateControlMoves() []Move {
	moves := make([]Move, 0, kDefaultMoveListLength)

	pinnedPieces := b.generatePinnedMoves(&moves)
	nonpinnedPieces := ^pinnedPieces

	// Finally, compute ordinary moves, ignoring absolutely pinned pieces on the board.
	b.pawnControls(&moves, nonpinnedPieces)
	b.knightControls(&moves, nonpinnedPieces)
	b.rookControls(&moves, nonpinnedPieces)
	b.bishopControls(&moves, nonpinnedPieces)
	b.queenControls(&moves, nonpinnedPieces)
	b.kingControls(&moves)
	return moves
}

// Pawn captures (non enpassant) - all squares
func (b *Board) pawnControls(moveList *[]Move, nonpinned uint64) {
	east, west := b.pawnControlsBitboards(nonpinned)

	east, west = east, west
	dirbitboards := [2]uint64{east, west}
	if !b.Wtomove {
		dirbitboards[0], dirbitboards[1] = dirbitboards[1], dirbitboards[0]
	}
	for dir, board := range dirbitboards { // for east and west
		for board != 0 {
			target := bits.TrailingZeros64(board)
			board &= board - 1
			var move Move
			move.Setto(Square(target))
			canPromote := false
			if b.Wtomove {
				move.Setfrom(Square(target - (9 - (dir * 2))))
				canPromote = target >= 56
			} else {
				move.Setfrom(Square(target + (9 - (dir * 2))))
				canPromote = target <= 7
			}
			
			if canPromote {
				move.Setpromote(Queen)
				*moveList = append(*moveList, move)
				continue
			}
			*moveList = append(*moveList, move)
		}
	}
}

// A helper than generates bitboards for available pawn captures.
func (b *Board) pawnControlsBitboards(nonpinned uint64) (east uint64, west uint64) {
	notHFile := uint64(0x7F7F7F7F7F7F7F7F)
	notAFile := uint64(0xFEFEFEFEFEFEFEFE)
	var targets uint64 = b.Black.All | b.White.All

	if b.Wtomove {
		ourpawns := b.White.Pawns & nonpinned
		east = ourpawns << 9 & notAFile & targets
		west = ourpawns << 7 & notHFile & targets
	} else {
		ourpawns := b.Black.Pawns & nonpinned
		east = ourpawns >> 7 & notAFile & targets
		west = ourpawns >> 9 & notHFile & targets
	}
	return
}



// Knight moves - all squares
func (b *Board) knightControls(moveList *[]Move, nonpinned uint64) {
	var ourKnights uint64
	if b.Wtomove {
		ourKnights = b.White.Knights & nonpinned
	} else {
		ourKnights = b.Black.Knights & nonpinned
	}
	for ourKnights != 0 {
		currentKnight := bits.TrailingZeros64(ourKnights)
		ourKnights &= ourKnights - 1
		targets := knightMasks[currentKnight]
		genMovesFromTargets(moveList, Square(currentKnight), targets)
	}
}

// Bishop moves - all squares, past queens, past bishops
func (b *Board) bishopControls(moveList *[]Move, nonpinned uint64) {
	var ourBishops, transparentPieces uint64
	if b.Wtomove {
		ourBishops = b.White.Bishops & nonpinned
		transparentPieces = b.White.Bishops & b.White.Queens
	} else {
		ourBishops = b.Black.Bishops & nonpinned
		transparentPieces = b.Black.Bishops & b.Black.Queens
	}
	allPieces := b.White.All | b.Black.All
	for ourBishops != 0 {
		currBishop := uint8(bits.TrailingZeros64(ourBishops))
		ourBishops &= ourBishops - 1
		targets := CalculateBishopMoveBitboard(currBishop, allPieces ^ transparentPieces)
		genMovesFromTargets(moveList, Square(currBishop), targets)
	}
}

// Rook moves - all squares, past rooks, past queens
func (b *Board) rookControls(moveList *[]Move, nonpinned uint64) {
	var ourRooks, transparentPieces uint64
	if b.Wtomove {
		ourRooks = b.White.Rooks & nonpinned
		transparentPieces = b.White.Rooks & b.White.Queens
	} else {
		ourRooks = b.Black.Rooks & nonpinned
		transparentPieces = b.Black.Rooks & b.Black.Queens
	}
	allPieces := b.White.All | b.Black.All
	for ourRooks != 0 {
		currRook := uint8(bits.TrailingZeros64(ourRooks))
		ourRooks &= ourRooks - 1
		targets := CalculateRookMoveBitboard(currRook, allPieces ^ transparentPieces)
		genMovesFromTargets(moveList, Square(currRook), targets)
	}
}

// Queen moves - all squares, past rooks, past bishops, past queens
func (b *Board) queenControls(moveList *[]Move, nonpinned uint64) {
	var ourQueens, transparentDiag, transparentHorz uint64
	if b.Wtomove {
		ourQueens = b.White.Queens & nonpinned
		transparentDiag = b.White.Bishops & b.White.Queens
		transparentHorz = b.White.Rooks & b.White.Queens
	} else {
		ourQueens = b.Black.Queens & nonpinned
		transparentDiag = b.Black.Bishops & b.Black.Queens
		transparentHorz = b.Black.Rooks & b.Black.Queens
	}
	allPieces := b.White.All | b.Black.All
	for ourQueens != 0 {
		currQueen := uint8(bits.TrailingZeros64(ourQueens))
		ourQueens &= ourQueens - 1
		// bishop motion
		diag_targets := CalculateBishopMoveBitboard(currQueen, allPieces ^ transparentDiag)
		genMovesFromTargets(moveList, Square(currQueen), diag_targets)
		// rook motion
		ortho_targets := CalculateRookMoveBitboard(currQueen, allPieces ^ transparentHorz)
		genMovesFromTargets(moveList, Square(currQueen), ortho_targets)
	}
}

// King moves (non castle)
// Computes king moves without castling.
func (b *Board) kingControls(moveList *[]Move) {
	if b.Wtomove {
		ourKing = b.White.Kings
	} else {
		ourKing = b.Black.Kings
	}

	ourKingLocation := uint8(bits.TrailingZeros64(ourKing))

	// TODO(dylhunn): Modifying the board is NOT thread-safe.
	// We only do this to avoid the king danger problem, aka moving away from a
	// checking slider.
	targets := kingMasks[ourKingLocation]
	for targets != 0 {
		target := bits.TrailingZeros64(targets)
		targets &= targets - 1
		var move Move
		move.Setfrom(Square(ourKingLocation)).Setto(Square(target))
		*moveList = append(*moveList, move)
	}
}