//go:build !race

package pdf

// slow scales the "hung" deadlines: the race detector runs code 5-10x slower.
const slow = 1
