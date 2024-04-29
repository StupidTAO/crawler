package generator

import (
	"fmt"
	"testing"
)

func TestIDbyIP(t *testing.T) {
	id := IDbyIP("127.0.0.2")
	fmt.Println("id = ", id)
}
