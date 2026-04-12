package commands

import (
	"testing"

	"github.com/red55/bgp-dns/internal/app"
	"github.com/stretchr/testify/assert"
)

func TestNewListCmd_HasReloadSubcommand(t *testing.T) {
	_app := &app.Application{}
	cmd := newListCmd(_app, nil)

	assert.Equal(t, "list", cmd.Use)
	assert.NotNil(t, cmd)

	// Verify reload subcommand exists
	reloadCmd, _, err := cmd.Find([]string{"reload"})
	assert.NoError(t, err)
	assert.NotNil(t, reloadCmd)
	assert.Equal(t, "reload", reloadCmd.Use)
}

func TestNewListCmd_HasAtLeastOneSubcommand(t *testing.T) {
	_app := &app.Application{}
	cmd := newListCmd(_app, nil)

	subcommands := cmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 1)
}
