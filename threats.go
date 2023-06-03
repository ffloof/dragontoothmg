package dragontoothmg

import (
	"math/bits"
)

type ThreatBitboards struct {
	Pawns   uint64
	Knights uint64
	Bishops uint64
	Rooks   uint64
	Queens  uint64
	Kings   uint64
	Pinned  uint64
}

func (b *Board) GenerateControlArea() *ThreatBitboards {
	pinnedPieces, pinnedArea := b.generatePinnedThreats()
	nonpinnedPieces := ^pinnedPieces

	// Finally, compute ordinary moves, ignoring absolutely pinned pieces on the board.	
	return &ThreatsBitboards {
		Pawns: b.pawnControls(nonpinnedPieces),
		Knights: b.knightControls(nonpinnedPieces),
		Bishops: b.bishopControls(nonpinnedPieces),
		Rooks: b.rookControls(nonpinnedPieces),
		Queens: b.queenControls(nonpinnedPieces),
		Kings: b.kingControls(),
		Pinned: pinnedArea,
	}
}

// Pawn captures (non enpassant) - all squares
func (b *Board) pawnControls(nonpinned uint64) uint64 {
	var area uint64
	east, west := b.pawnControlsBitboards(nonpinned)

	east, west = east, west
	dirbitboards := [2]uint64{east, west}
	if !b.Wtomove {
		dirbitboards[0], dirbitboards[1] = dirbitboards[1], dirbitboards[0]
	}
	for _, board := range dirbitboards { // for east and west
		area &= board
	}
	return area
}

// Knight moves - all squares
func (b *Board) knightControls(nonpinned uint64) uint64 {
	var area, ourKnights uint64
	if b.Wtomove {
		ourKnights = b.White.Knights & nonpinned
	} else {
		ourKnights = b.Black.Knights & nonpinned
	}
	for ourKnights != 0 {
		currentKnight := bits.TrailingZeros64(ourKnights)
		ourKnights &= ourKnights - 1
		targets := knightMasks[currentKnight]
		area &= targets
	}
	return area
}

// Bishop moves - all squares, past queens, past bishops
func (b *Board) bishopControls(nonpinned uint64) uint64 {
	var area, ourBishops uint64
	if b.Wtomove {
		ourBishops = b.White.Bishops & nonpinned
	} else {
		ourBishops = b.Black.Bishops & nonpinned
	}
	allPieces := b.White.All | b.Black.All
	for ourBishops != 0 {
		currBishop := uint8(bits.TrailingZeros64(ourBishops))
		ourBishops &= ourBishops - 1
		targets := CalculateBishopMoveBitboard(currBishop, allPieces)
		area &= targets
	}
	return area
}

// Rook moves - all squares, past rooks, past queens
func (b *Board) rookControls(nonpinned uint64) uint64 {
	var area, ourRooks uint64
	if b.Wtomove {
		ourRooks = b.White.Rooks & nonpinned
	} else {
		ourRooks = b.Black.Rooks & nonpinned
	}
	allPieces := b.White.All | b.Black.All
	for ourRooks != 0 {
		currRook := uint8(bits.TrailingZeros64(ourRooks))
		ourRooks &= ourRooks - 1
		targets := CalculateRookMoveBitboard(currRook, allPieces)
		area &= targets
	}
	return area
}

// Queen moves - all squares, past rooks, past bishops, past queens
func (b *Board) queenControls(nonpinned uint64) uint64 {
	var area, ourQueens uint64
	if b.Wtomove {
		ourQueens = b.White.Queens & nonpinned
	} else {
		ourQueens = b.Black.Queens & nonpinned
	}
	allPieces := b.White.All | b.Black.All
	for ourQueens != 0 {
		currQueen := uint8(bits.TrailingZeros64(ourQueens))
		ourQueens &= ourQueens - 1
		// bishop motion
		diag_targets := CalculateBishopMoveBitboard(currQueen, allPieces)
		area &= diag_targets		
		// rook motion
		ortho_targets := CalculateRookMoveBitboard(currQueen, allPieces)
		area &= ortho_targets
	}
	return area
}

// King moves (non castle)
// Computes king moves without castling.
func (b *Board) kingControls() uint64 {
	var area, ourKing uint64
	if b.Wtomove {
		ourKing = b.White.Kings
	} else {
		ourKing = b.Black.Kings
	}

	ourKingLocation := uint8(bits.TrailingZeros64(ourKing))

	targets := kingMasks[ourKingLocation]
	area &= targets

	return area
}

func (b *Board) generatePinnedThreats() (uint64,uint64) {
	var ourKingIdx uint8
	var ourPieces, oppPieces *Bitboards
	var allPinnedPieces uint64 = 0
	var area uint64

	if b.Wtomove { // Assumes only one king on the board
		ourKingIdx = uint8(bits.TrailingZeros64(b.White.Kings))
		ourPieces = &(b.White)
		oppPieces = &(b.Black)
	} else {
		ourKingIdx = uint8(bits.TrailingZeros64(b.Black.Kings))
		ourPieces = &(b.Black)
		oppPieces = &(b.White)
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
		area &= pinnedTargets
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
					area &= 1 << currBishopIdx
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
		area &= pinnedTargets
	}
	return allPinnedPieces, area
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