package fake_test

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	common "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/events"
	fake "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/fake"
	msg "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/msg"
)

func TestParseHeader(t *testing.T) {
	p := fake.NewParser()
	expected := common.DemoHeader{
		Filestamp:      "HL2DEMO",
		MapName:        "de_cache",
		PlaybackFrames: 64 * 1000,
		PlaybackTicks:  128 * 1000,
		PlaybackTime:   time.Second * 1000,
	}
	p.On("ParseHeader").Return(expected, nil)

	actual, err := p.ParseHeader()

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseNextFrameEvents(t *testing.T) {
	p := fake.NewParser()
	p.On("ParseNextFrame").Return(true, nil)

	expected := []any{kill(common.EqAK47), kill(common.EqScout)}
	p.MockEvents(expected...)

	// Kill on second frame that shouldn't be dispatched during the first frame
	p.MockEvents(kill(common.EqAUG))

	var actual []any
	p.RegisterEventHandler(func(e events.Kill) {
		actual = append(actual, e)
	})

	next, err := p.ParseNextFrame()

	assert.True(t, next)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func kill(wepType common.EquipmentType) events.Kill {
	wep := common.NewEquipment(wepType)
	return events.Kill{
		Killer: new(common.Player),
		Weapon: wep,
		Victim: new(common.Player),
	}
}

func TestParseToEndEvents(t *testing.T) {
	p := fake.NewParser()
	p.On("ParseToEnd").Return(nil)
	expected := []any{kill(common.EqAK47), kill(common.EqScout), kill(common.EqAUG)}
	p.MockEvents(expected[:1]...)
	p.MockEvents(expected[1:]...)

	var actual []any
	p.RegisterEventHandler(func(e events.Kill) {
		actual = append(actual, e)
	})

	err := p.ParseToEnd()

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseNextFrameNetMessages(t *testing.T) {
	p := fake.NewParser()
	p.On("ParseNextFrame").Return(true, nil)
	expected := []any{
		cmdKey(1, 2, 3),
		cmdKey(100, 255, 8),
	}

	p.MockNetMessages(expected...)
	// Message on second frame that shouldn't be dispatched during the first frame
	p.MockNetMessages(msg.CSVCMsg_Menu{DialogType: proto.Int32(1), MenuKeyValues: []byte{1, 55, 99}})

	var actual []any
	p.RegisterNetMessageHandler(func(message any) {
		actual = append(actual, message)
	})

	next, err := p.ParseNextFrame()

	assert.True(t, next)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseToEndNetMessages(t *testing.T) {
	p := fake.NewParser()
	p.On("ParseToEnd").Return(nil)
	expected := []any{
		cmdKey(1, 2, 3),
		cmdKey(100, 255, 8),
		msg.CSVCMsg_Menu{DialogType: proto.Int32(1), MenuKeyValues: []byte{1, 55, 99}},
	}

	p.MockNetMessages(expected[:1]...)
	p.MockNetMessages(expected[1:]...)

	var actual []any
	p.RegisterNetMessageHandler(func(message any) {
		actual = append(actual, message)
	})

	err := p.ParseToEnd()

	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func cmdKey(b ...byte) msg.CSVCMsg_CmdKeyValues {
	return msg.CSVCMsg_CmdKeyValues{
		Keyvalues: b,
	}
}
