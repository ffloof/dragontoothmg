package dragontoothmg

// Applies a move to the board, and returns a function that can be used to unapply it.
// This function assumes that the given move is valid (i.e., is in the set of moves found by GenerateLegalMoves()).
// If the move is not valid, this function has undefined behavior.
func (b *Board) Apply(m Move) {
	// Configure data about which pieces move
	var ourBitboardPtr, oppBitboardPtr *Bitboards
	var epDelta int8                                // add this to the e.p. square to find the captured pawn
	var oppStartingRankBb, ourStartingRankBb uint64 // the starting rank of out opponent's major pieces
	// the constant that represents the index into pieceSquareZobristC for the pawn of our color
	var ourPiecesPawnZobristIndex int
	var oppPiecesPawnZobristIndex int
	if b.Wtomove {
		ourBitboardPtr = &(b.White)
		oppBitboardPtr = &(b.Black)
		epDelta = -8
		oppStartingRankBb = onlyRank[7]
		ourStartingRankBb = onlyRank[0]
		ourPiecesPawnZobristIndex = 0
		oppPiecesPawnZobristIndex = 6
	} else {
		ourBitboardPtr = &(b.Black)
		oppBitboardPtr = &(b.White)
		epDelta = 8
		oppStartingRankBb = onlyRank[0]
		ourStartingRankBb = onlyRank[7]
		b.Fullmoveno++ // increment after black's move
		ourPiecesPawnZobristIndex = 6
		oppPiecesPawnZobristIndex = 0
	}
	fromBitboard := (uint64(1) << m.From())
	toBitboard := (uint64(1) << m.To())
	pieceType, pieceTypeBitboard := determinePieceType(ourBitboardPtr, fromBitboard)
	castleStatus := 0
	var oldRookLoc, newRookLoc uint8

	// If it is any kind of capture or pawn move, reset halfmove clock.
	if IsCapture(m, b) || pieceType == Pawn { 
		b.Halfmoveclock = 0 // reset halfmove clock
	} else {
		b.Halfmoveclock++
	}

	// King moves strip castling rights
	if pieceType == King {
		// TODO(dylhunn): do this without a branch
		if m.To()-m.From() == 2 { // castle short
			castleStatus = 1
			oldRookLoc = m.To() + 1
			newRookLoc = m.To() - 1
		} else if int(m.To())-int(m.From()) == -2 { // castle long
			castleStatus = -1
			oldRookLoc = m.To() - 2
			newRookLoc = m.To() + 1
		}
		// King moves always strip castling rights
		if b.canCastleKingside() {
			b.flipKingsideCastle()
		}
		if b.canCastleQueenside() {
			b.flipQueensideCastle()
		}
	}

	// Rook moves strip castling rights
	if pieceType == Rook {
		if b.canCastleKingside() && (fromBitboard&onlyFile[7] != 0) &&
			fromBitboard&ourStartingRankBb != 0 { // king's rook
			b.flipKingsideCastle()
		} else if b.canCastleQueenside() && (fromBitboard&onlyFile[0] != 0) &&
			fromBitboard&ourStartingRankBb != 0 { // queen's rook
			b.flipQueensideCastle()
		}
	}

	// Apply the castling rook movement
	if castleStatus != 0 {
		ourBitboardPtr.Rooks |= (uint64(1) << newRookLoc)
		ourBitboardPtr.All |= (uint64(1) << newRookLoc)
		ourBitboardPtr.Rooks &= ^(uint64(1) << oldRookLoc)
		ourBitboardPtr.All &= ^(uint64(1) << oldRookLoc)
		// Update rook location in hash
		// (Rook - 1) assumes that "Nothing" precedes "Rook" in the Piece constants list
		b.hash ^= pieceSquareZobristC[ourPiecesPawnZobristIndex+(Rook-1)][oldRookLoc]
		b.hash ^= pieceSquareZobristC[ourPiecesPawnZobristIndex+(Rook-1)][newRookLoc]
	}

	// Is this an e.p. capture? Strip the opponent pawn and reset the e.p. square
	oldEpCaptureSquare := b.Enpassant
	if pieceType == Pawn && m.To() == oldEpCaptureSquare && oldEpCaptureSquare != 0 {
		epOpponentPawnLocation := uint8(int8(oldEpCaptureSquare) + epDelta)
		oppBitboardPtr.Pawns &= ^(uint64(1) << epOpponentPawnLocation)
		oppBitboardPtr.All &= ^(uint64(1) << epOpponentPawnLocation)
		// Remove the opponent pawn from the board hash.
		b.hash ^= pieceSquareZobristC[oppPiecesPawnZobristIndex][epOpponentPawnLocation]
	}
	// Update the en passant square
	if pieceType == Pawn && (int8(m.To())+2*epDelta == int8(m.From())) { // pawn double push
		b.Enpassant = uint8(int8(m.To()) + epDelta)
	} else {
		b.Enpassant = 0
	}

	// Is this a promotion?
	var destTypeBitboard *uint64
	var promotedToPieceType Piece // if not promoted, same as pieceType
	switch m.Promote() {
	case Queen:
		destTypeBitboard = &(ourBitboardPtr.Queens)
		promotedToPieceType = Queen
	case Knight:
		destTypeBitboard = &(ourBitboardPtr.Knights)
		promotedToPieceType = Knight
	case Rook:
		destTypeBitboard = &(ourBitboardPtr.Rooks)
		promotedToPieceType = Rook
	case Bishop:
		destTypeBitboard = &(ourBitboardPtr.Bishops)
		promotedToPieceType = Bishop
	default:
		destTypeBitboard = pieceTypeBitboard
		promotedToPieceType = pieceType
	}

	// Apply the move
	capturedPieceType, capturedBitboard := determinePieceType(oppBitboardPtr, toBitboard)
	ourBitboardPtr.All &= ^fromBitboard // remove at "from"
	ourBitboardPtr.All |= toBitboard    // add at "to"
	*pieceTypeBitboard &= ^fromBitboard // remove at "from"
	*destTypeBitboard |= toBitboard     // add at "to"
	if capturedPieceType != Nothing {   // This does not account for e.p. captures
		*capturedBitboard &= ^toBitboard
		oppBitboardPtr.All &= ^toBitboard
		b.hash ^= pieceSquareZobristC[oppPiecesPawnZobristIndex+(int(capturedPieceType)-1)][m.To()] // remove the captured piece from the hash
	}
	b.hash ^= pieceSquareZobristC[(int(pieceType)-1)+ourPiecesPawnZobristIndex][m.From()]         // remove piece at "from"
	b.hash ^= pieceSquareZobristC[(int(promotedToPieceType)-1)+ourPiecesPawnZobristIndex][m.To()] // add piece at "to"

	// If a rook was captured, it strips castling rights
	if capturedPieceType == Rook {
		if m.To()%8 == 7 && toBitboard&oppStartingRankBb != 0 && b.oppCanCastleKingside() { // captured king rook
			b.flipOppKingsideCastle()
		} else if m.To()%8 == 0 && toBitboard&oppStartingRankBb != 0 && b.oppCanCastleQueenside() { // queen rooks
			b.flipOppQueensideCastle()
		}
	}
	// flip the side to move in the hash
	b.hash ^= whiteToMoveZobristC
	b.Wtomove = !b.Wtomove

	// remove the old en passant square from the hash, and add the new one
	b.hash ^= uint64(oldEpCaptureSquare)
	b.hash ^= uint64(b.Enpassant)
}

func determinePieceType(ourBitboardPtr *Bitboards, squareMask uint64) (Piece, *uint64) {
	var pieceType Piece = Nothing
	pieceTypeBitboard := &(ourBitboardPtr.All)
	if squareMask&ourBitboardPtr.Pawns != 0 {
		pieceType = Pawn
		pieceTypeBitboard = &(ourBitboardPtr.Pawns)
	} else if squareMask&ourBitboardPtr.Knights != 0 {
		pieceType = Knight
		pieceTypeBitboard = &(ourBitboardPtr.Knights)
	} else if squareMask&ourBitboardPtr.Bishops != 0 {
		pieceType = Bishop
		pieceTypeBitboard = &(ourBitboardPtr.Bishops)
	} else if squareMask&ourBitboardPtr.Rooks != 0 {
		pieceType = Rook
		pieceTypeBitboard = &(ourBitboardPtr.Rooks)
	} else if squareMask&ourBitboardPtr.Queens != 0 {
		pieceType = Queen
		pieceTypeBitboard = &(ourBitboardPtr.Queens)
	} else if squareMask&ourBitboardPtr.Kings != 0 {
		pieceType = King
		pieceTypeBitboard = &(ourBitboardPtr.Kings)
	}
	return pieceType, pieceTypeBitboard
}
