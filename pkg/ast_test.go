package pkg

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMains(t *testing.T) {
	binaries := Binaries("..", Resolution{
		LocalPrefix:  "github.com/paradoxical-io",
		IncludeTests: true,
	})

	for _, bin := range binaries {
		fmt.Println(bin.BinaryName)
		for _, dep := range bin.Dependencies {
			fmt.Println("  " + dep.ImportValue)
		}
	}

	assert.Equal(t, 3, len(binaries))
}
