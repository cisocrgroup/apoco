// Code generated by "stringer -type StokCause"; DO NOT EDIT.

package internal

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[BadRank-0]
	_ = x[BadLimit-1]
	_ = x[MissingCandidate-2]
}

const _StokCause_name = "BadRankBadLimitMissingCandidate"

var _StokCause_index = [...]uint8{0, 7, 15, 31}

func (i StokCause) String() string {
	if i < 0 || i >= StokCause(len(_StokCause_index)-1) {
		return "StokCause(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _StokCause_name[_StokCause_index[i]:_StokCause_index[i+1]]
}