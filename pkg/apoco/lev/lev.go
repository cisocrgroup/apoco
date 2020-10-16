package lev

// Distance calculates the levenshtein distance between s1 and s2.
// Reference: [Levenshtein
// Distance](http://en.wikipedia.org/wiki/Levenshtein_distance).
//
// Note that this calculation's result isn't normalized. (not between
// 0 and 1.)  and if s1 and s2 are exactly the same, the result is 0.
func Distance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	r1 := []rune(s1)
	r2 := []rune(s2)
	if len(r1) == 0 {
		return len(r2)
	}
	if len(r2) == 0 {
		return len(r1)
	}
	m := make([][]int, len(r1)+1)
	for i := range m {
		m[i] = make([]int, len(r2)+1)
	}
	for i := 0; i < len(r1)+1; i++ {
		for j := 0; j < len(r2)+1; j++ {
			if i == 0 {
				m[i][j] = j
			} else if j == 0 {
				m[i][j] = i
			} else {
				if r1[i-1] == r2[j-1] {
					m[i][j] = m[i-1][j-1]
				} else {
					m[i][j] = min(m[i-1][j]+1, m[i][j-1]+1, m[i-1][j-1]+1)
				}
			}
		}
	}
	return m[len(r1)][len(r2)]
}

func min(min int, is ...int) int {
	for _, v := range is {
		if v < min {
			min = v
		}
	}
	return min
}
