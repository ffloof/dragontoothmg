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

	pinnedPieces := b.generatePinnedThreats(&moves)
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

	if b.Wtomove {
		ourpawns := b.White.Pawns & nonpinned
		east = ourpawns << 9 & notAFile
		west = ourpawns << 7 & notHFile
	} else {
		ourpawns := b.Black.Pawns & nonpinned
		east = ourpawns >> 7 & notAFile
		west = ourpawns >> 9 & notHFile
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
	var ourKing uint64
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

func (b *Board) generatePinnedThreats(moveList *[]Move) uint64 {
	var ourKingIdx uint8
	var ourPieces, oppPieces *Bitboards
	var allPinnedPieces uint64 = 0
	var ourPromotionRank uint64
	if b.Wtomove { // Assumes only one king on the board
		ourKingIdx = uint8(bits.TrailingZeros64(b.White.Kings))
		ourPieces = &(b.White)
		oppPieces = &(b.Black)
		ourPromotionRank = onlyRank[7]
	} else {
		ourKingIdx = uint8(bits.TrailingZeros64(b.Black.Kings))
		ourPieces = &(b.Black)
		oppPieces = &(b.White)
		ourPromotionRank = onlyRank[0]
	}
	allPieces := oppPieces.All | ourPieces.All

	// Calculate king moves as if it was a rook.
	// "king targets" includes our own friendly pieces, for the purpose of identifying pins.
	kingOrthoTargets := CalculateRookMoveBitboard(ourKingIdx, allPieces)
	oppRooks := oppPieces.Rooks | oppPieces.Queens
	for oppRooks != 0 { // For each opponent ortho slider
		currRookIdx := uint8(bits.TrailingZeros64(oppRooks))
		oppRooks &= oppRooks - 1
		rookTargets := CalculateRookMoveBitboard(currRookIdx, allPieces) & (^(oppPieces.All))
		// A piece is pinned iff it falls along both attack rays.
		pinnedPiece := rookTargets & kingOrthoTargets & ourPieces.All
		if pinnedPiece == 0 { // there is no pin
			continue
		}
		pinnedPieceIdx := uint8(bits.TrailingZeros64(pinnedPiece))
		sameRank := pinnedPieceIdx/8 == ourKingIdx/8 && pinnedPieceIdx/8 == currRookIdx/8
		sameFile := pinnedPieceIdx%8 == ourKingIdx%8 && pinnedPieceIdx%8 == currRookIdx%8
		if !sameRank && !sameFile {
			continue // it's just an intersection, not a pin
		}
		allPinnedPieces |= pinnedPiece        // store the pinned piece location
		
		// If it's not a rook or queen, it can't move
		if pinnedPiece&ourPieces.Rooks == 0 && pinnedPiece&ourPieces.Queens == 0 {
			continue
		}
		// all ortho moves, as if it was not pinned
		pinnedPieceAllMoves := CalculateRookMoveBitboard(pinnedPieceIdx, allPieces) & (^(ourPieces.All))
		// actually available moves
		pinnedTargets := pinnedPieceAllMoves & (rookTargets | kingOrthoTargets | (uint64(1) << currRookIdx))
			genMovesFromTargets(moveList, Square(pinnedPieceIdx), pinnedTargets)
	}

	// Calculate king moves as if it was a bishop.
	// "king targets" includes our own friendly pieces, for the purpose of identifying pins.
	kingDiagTargets := CalculateBishopMoveBitboard(ourKingIdx, allPieces)
	oppBishops := oppPieces.Bishops | oppPieces.Queens
	for oppBishops != 0 {
		currBishopIdx := uint8(bits.TrailingZeros64(oppBishops))
		oppBishops &= oppBishops - 1
		bishopTargets := CalculateBishopMoveBitboard(currBishopIdx, allPieces) & (^(oppPieces.All))
		pinnedPiece := bishopTargets & kingDiagTargets & ourPieces.All
		if pinnedPiece == 0 { // there is no pin
			continue
		}
		pinnedPieceIdx := uint8(bits.TrailingZeros64(pinnedPiece))
		bishopToPinnedSlope := (float32(pinnedPieceIdx)/8 - float32(currBishopIdx)/8) /
			(float32(pinnedPieceIdx%8) - float32(currBishopIdx%8))
		bishopToKingSlope := (float32(ourKingIdx)/8 - float32(currBishopIdx)/8) /
			(float32(ourKingIdx%8) - float32(currBishopIdx%8))
		if bishopToPinnedSlope != bishopToKingSlope { // just an intersection, not a pin
			continue
		}
		allPinnedPieces |= pinnedPiece // store pinned piece
		// if it's a pawn we might be able to capture with it
		if pinnedPiece&ourPieces.Pawns != 0 {
			if (uint64(1)<<currBishopIdx) != 0 {
				if (b.Wtomove && (pinnedPieceIdx/8)+1 == currBishopIdx/8) ||
					(!b.Wtomove && pinnedPieceIdx/8 == (currBishopIdx/8)+1) {
					if ((uint64(1) << currBishopIdx) & ourPromotionRank) != 0 { // We get to promote!
						for i := Piece(Knight); i <= Queen; i++ {
							var move Move
							move.Setfrom(Square(pinnedPieceIdx)).Setto(Square(currBishopIdx)).Setpromote(i)
							*moveList = append(*moveList, move)
						}
					} else { // no promotion
						var move Move
						move.Setfrom(Square(pinnedPieceIdx)).Setto(Square(currBishopIdx))
						*moveList = append(*moveList, move)
					}
				}
			}
			continue
		}
		// If it's not a bishop or queen, it can't move
		if pinnedPiece&ourPieces.Bishops == 0 && pinnedPiece&ourPieces.Queens == 0 {
			continue
		}
		// all diag moves, as if it was not pinned
		pinnedPieceAllMoves := CalculateBishopMoveBitboard(pinnedPieceIdx, allPieces) & (^(ourPieces.All))
		// actually available moves
		pinnedTargets := pinnedPieceAllMoves & (bishopTargets | kingDiagTargets | (uint64(1) << currBishopIdx))
		genMovesFromTargets(moveList, Square(pinnedPieceIdx), pinnedTargets)
	}
	return allPinnedPieces
}
