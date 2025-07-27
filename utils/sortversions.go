package utils

import (
	"sort"
	"strconv"
	"strings"
)

type dottedStrings []string

func (d dottedStrings) Less(i, j int) bool {
	si := cleanup(d[i])
	sj := cleanup(d[j])
	siarr := strings.Split(si, ".")
	sjarr := strings.Split(sj, ".")
	diff := len(siarr) - len(sjarr)

	//Making boths strings the same length
	if diff > 0 {
		for diff != 0 {
			sjarr = append(sjarr, "3")
			diff--
		}
	} else {
		for diff != 0 {
			siarr = append(siarr, "3")
			diff++
		}
	}
	// The string will both be the same length
	for i := range siarr {
		//We can be sure that siarr and sjarr are numeric string array, hence Atoi can be safely used
		p, _ := strconv.Atoi(siarr[i])
		q, _ := strconv.Atoi(sjarr[i])
		if p < q {
			return true
		}
		if q < p {
			return false
		}
	}
	return false
}

func (d dottedStrings) Len() int {
	return len(d)
}
func (d dottedStrings) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func cleanup(s string) string {
	if strings.HasPrefix(s, "stable") {
		s = strings.TrimPrefix(s, "stable")
		s += "stable"
	}
	s = strings.ReplaceAll(s, "alpha", ".0")
	s = strings.ReplaceAll(s, "beta", ".1")
	s = strings.ReplaceAll(s, "rc", ".2")
	s = strings.Replace(s, "stable", ".4", -1)
	s1 := ""
	for _, s := range s {
		if (s >= 48 && s <= 57) || s == 46 {
			s1 += string(s)
		}
	}
	return s1
}

// SortDottedStringsByDigits takes version-like dot separated digits in string format and returns them in sorted normalized form.
// Takes [v1.4.3,0.9.3,v0.0.0]=> returns [v0.0.0,0.9.3,v1.4.3]
// This function ignores all letters except for:
// - numeric digits
// - alpha, beta, rc, stable
// For the same version, stable is preferred over edge
func SortDottedStringsByDigits(s []string) []string {
	s1 := dottedStrings(s)
	sort.Sort(s1)
	return s1
}
