package demoinfocs

import (
	"bytes"
	"encoding/binary"

	"github.com/markus-wa/ice-cipher-go/pkg/ice"
	"google.golang.org/protobuf/proto"

	bit "github.com/markus-wa/demoinfocs-golang/v3/internal/bitread"
	events "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/events"
	msg "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/msg"
)

func (p *parser) handlePacketEntities(pe *msg.CSVCMsg_PacketEntities) {
	defer func() {
		p.setError(recoverFromUnexpectedEOF(recover()))
	}()

	r := bit.NewSmallBitReader(bytes.NewReader(pe.EntityData))

	currentEntity := -1
	for i := 0; i < int(pe.GetUpdatedEntries()); i++ {
		currentEntity += 1 + int(r.ReadUBitInt())

		cmd := r.ReadBitsToByte(2)
		if cmd&1 == 0 {
			if cmd&2 != 0 {
				// Enter PVS
				if existing := p.gameState.entities[currentEntity]; existing != nil {
					// Sometimes entities don't get destroyed when they should be
					// For instance when a player is replaced by a BOT
					existing.Destroy()
				}

				p.gameState.entities[currentEntity] = p.stParser.ReadEnterPVS(r, currentEntity)
			} else {
				// Delta Update
				if entity := p.gameState.entities[currentEntity]; entity != nil {
					entity.ApplyUpdate(r)
				}
			}
		} else {
			if cmd&2 != 0 {
				// Leave PVS
				if entity := p.gameState.entities[currentEntity]; entity != nil {
					entity.Destroy()
					delete(p.gameState.entities, currentEntity)
				}
			}
		}
	}

	p.poolBitReader(r)
}

func (p *parser) handleSetConVar(setConVar *msg.CNETMsg_SetConVar) {
	updated := make(map[string]string)
	for _, cvar := range setConVar.Convars.Cvars {
		updated[cvar.GetName()] = cvar.GetValue()
		p.gameState.rules.conVars[cvar.GetName()] = cvar.GetValue()
	}

	p.eventDispatcher.Dispatch(events.ConVarsUpdated{
		UpdatedConVars: updated,
	})
}

func (p *parser) handleServerInfo(srvInfo *msg.CSVCMsg_ServerInfo) {
	// srvInfo.MapCrc might be interesting as well
	p.tickInterval = srvInfo.GetTickInterval()

	p.eventDispatcher.Dispatch(events.TickRateInfoAvailable{
		TickRate: p.TickRate(),
		TickTime: p.TickTime(),
	})
}

func (p *parser) handleEncryptedData(msg *msg.CSVCMsg_EncryptedData) {
	if msg.GetKeyType() != 2 {
		return
	}

	if p.decryptionKey == nil {
		p.msgDispatcher.Dispatch(events.ParserWarn{
			Type:    events.WarnTypeMissingNetMessageDecryptionKey,
			Message: "received encrypted net-message but no decryption key is set",
		})

		return
	}

	k := ice.NewKey(2, p.decryptionKey)
	b := k.DecryptAll(msg.Encrypted)

	r := bytes.NewReader(b)
	br := bit.NewSmallBitReader(r)

	const (
		byteLenPadding = 1
		byteLenWritten = 4
	)

	paddingBytes := br.ReadSingleByte()

	if int(paddingBytes) >= len(b)-byteLenPadding-byteLenWritten {
		p.eventDispatcher.Dispatch(events.ParserWarn{
			Message: "encrypted net-message has invalid number of padding bytes",
			Type:    events.WarnTypeCantReadEncryptedNetMessage,
		})

		return
	}

	br.Skip(int(paddingBytes) << 3)

	bBytesWritten := br.ReadBytes(4)
	nBytesWritten := int(binary.BigEndian.Uint32(bBytesWritten))

	if len(b) != byteLenPadding+byteLenWritten+int(paddingBytes)+nBytesWritten {
		p.eventDispatcher.Dispatch(events.ParserWarn{
			Message: "encrypted net-message has invalid length",
			Type:    events.WarnTypeCantReadEncryptedNetMessage,
		})

		return
	}

	cmd := br.ReadVarInt32()
	size := br.ReadVarInt32()

	m := p.netMessageForCmd(int(cmd))

	if m == nil {
		err := br.Pool()
		if err != nil {
			p.setError(err)
		}

		return
	}

	msgB := br.ReadBytes(int(size))

	err := proto.Unmarshal(msgB, m)
	if err != nil {
		p.setError(err)

		return
	}

	p.msgDispatcher.Dispatch(m)
}
