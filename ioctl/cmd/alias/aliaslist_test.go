package alias

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// test aliasListMessage print string
func TestAliasListMessage_String(t *testing.T) {
	require := require.New(t)
	message := aliasListMessage{AliasNumber: 3}
	message.AliasList = append(message.AliasList, alias{Name: "a", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.AliasList = append(message.AliasList, alias{Name: "b", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.AliasList = append(message.AliasList, alias{Name: "c", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1"})
	str := "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx - a\nio1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx - b\nio1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1 - c"
	require.Equal(message.String(), str)
}

// test alias list cmd
func TestAliasListCmd(t *testing.T) {
	c, output, err := executeCommandC(AliasCmd, "list")
	if output != "" {
		t.Errorf("Unexpected output: %v", output)
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if c.Name() != "list" {
		t.Errorf(`invalid command returned from ExecuteC: expected "list"', got %q`, c.Name())
	}
}
