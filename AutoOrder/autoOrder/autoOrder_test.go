package autoOrder

import (
	"fmt"
	"math"
	"testing"
)

func TestConvertInt8ToRune(t *testing.T) {
	var num int8 = 78

	// Intentional conversion to rune.
	str := fmt.Sprintf("%c", num)
	if str != "N" {
		t.Errorf("Expected 'N', but got '%s'", str)
	}

	t.Logf(str)
	t.Logf(string(num))

	fmt.Println(math.MaxInt8)
}
